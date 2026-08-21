// ====== 主题管理 (提前执行避免闪烁) ======
const THEME_KEY = 'easynotes-theme';
ace.config.set('basePath', '/assets/vendor/ace');

function getStoredTheme() { return localStorage.getItem(THEME_KEY) || 'auto'; }

function resolveTheme(theme) {
    if (theme === 'auto') return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    return theme;
}

function getVditorThemeName(resolved) { return resolved === 'dark' ? 'dark' : 'classic'; }

// 立即应用主题
(function applyInitialTheme() {
    const resolved = resolveTheme(getStoredTheme());
    document.documentElement.setAttribute('data-theme', resolved);
})();

// 监听浏览器主题变化 (auto 模式下自动跟随)
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (getStoredTheme() === 'auto') {
        const resolved = resolveTheme('auto');
        document.documentElement.setAttribute('data-theme', resolved);
        if (window.vditorInstance) {
            try { window.vditorInstance.setTheme(getVditorThemeName(resolved)); } catch(e) {}
        }
        if (window.aceEditorInstance) {
            window.aceEditorInstance.setTheme(resolved === 'dark' ? 'ace/theme/tomorrow_night' : 'ace/theme/textmate');
        }
        updateThemeToggleIcon();
    }
});

function updateThemeToggleIcon() {
    const btn = document.getElementById('theme-toggle');
    if (!btn) return;
    const stored = getStoredTheme();
    if (stored === 'auto') btn.textContent = '🖥';
    else if (resolveTheme(stored) === 'dark') btn.textContent = '🌙';
    else btn.textContent = '☀';
}

function cycleTheme() {
    const current = getStoredTheme();
    const next = current === 'light' ? 'dark' : current === 'dark' ? 'auto' : 'light';
    localStorage.setItem(THEME_KEY, next);
    const resolved = resolveTheme(next);
    document.documentElement.setAttribute('data-theme', resolved);
    // 同步 Vditor 主题
    if (window.vditorInstance) {
        try { window.vditorInstance.setTheme(getVditorThemeName(resolved)); } catch(e) {}
    }
    if (window.aceEditorInstance) {
        window.aceEditorInstance.setTheme(resolved === 'dark' ? 'ace/theme/tomorrow_night' : 'ace/theme/textmate');
    }
    updateThemeToggleIcon();
}

$(function () {
    const BASE_URL = "";
    let currentSearchTerm = '';
    let searchTimeout = null;
    let vditor = null;
    let aceEditor = null;
    let currentFile = null;
    let saveInterval = null;
    let isDirty = false;
    let lastSaveTime = null;
    let isResizing = false;

    initResizeHandler();

    // 绑定主题切换按钮
    updateThemeToggleIcon();
    $('#theme-toggle').on('click', cycleTheme);

    // ====== 认证 ======
    function checkAuth() {
        const token = localStorage.getItem('token');
        if (!token) { redirectToLogin(); return false; }
        return true;
    }

    function redirectToLogin() { window.location.href = '/login'; }

    function handleUnauthorized() {
        localStorage.removeItem('token');
        $('#vditor-container').html(`
            <div class="unauthorized">
                <div class="unauthorized-msg">登录已过期，请重新登录</div>
                <button class="btn-primary" id="login-button">前往登录</button>
            </div>
        `);
        $('#login-button').on('click', redirectToLogin);
        destroyActiveEditor();
        currentFile = null; isDirty = false;
        setTimeout(redirectToLogin, 2000);
    }

    function getAuthHeaders() {
        const token = localStorage.getItem('token');
        return token ? { 'Authorization': 'Bearer ' + token } : {};
    }

    function authAjax(options) {
        if (!checkAuth()) return Promise.reject('未认证');
        const token = localStorage.getItem('token');
        options.headers = options.headers || {};
        if (token) options.headers['Authorization'] = 'Bearer ' + token;
        return new Promise((resolve, reject) => {
            $.ajax({
                ...options,
                statusCode: {
                    401: function () { handleUnauthorized(); reject('未授权'); }
                },
                success: function (data, ts, xhr) { resolve(data, ts, xhr); },
                error: function (xhr, ts, err) {
                    if (xhr.status === 401) handleUnauthorized();
                    reject(xhr, ts, err);
                }
            });
        });
    }

    // ====== 搜索 ======
    function updateSearchStats(count) {
        if (currentSearchTerm.length === 0) {
            $('#search-stats').text('');
        } else {
            $('#search-stats').text(`找到 ${count} 个匹配项`);
        }
    }

    $('#search-input').on('input', function () {
        const searchTerm = $(this).val().trim();
        currentSearchTerm = searchTerm;
        clearTimeout(searchTimeout);
        const effectiveTerm = searchTerm.startsWith('/') ? searchTerm.slice(1).trim() : searchTerm;
        if (searchTerm.length > 0 && effectiveTerm.replace(/\s+/g, '').length < 2) {
            $('#search-stats').text('请输入至少2个有效字符');
            return;
        }
        searchTimeout = setTimeout(() => {
            tree.jstree(true).refresh();
            if (searchTerm.length === 0) $('#search-stats').text('');
            else $('#search-stats').text('搜索中...');
        }, 300);
    });

    // ====== jsTree ======
    const tree = $('#file-tree').jstree({
        'core': {
            'data': function (obj, callback) {
                // 过滤掉根目录下的 _images 文件夹
                function filterTree(nodes, isRoot) {
                    if (!Array.isArray(nodes)) return nodes;
                    return nodes.filter(n => {
                        // 只在根层过滤 _images
                        if (isRoot && n.text === '_images') return false;
                        return true;
                    }).map(n => {
                        if (n.children && Array.isArray(n.children)) n.children = filterTree(n.children, false);
                        return n;
                    });
                }
                if (currentSearchTerm) {
                    authAjax({ url: BASE_URL + '/api/files', method: 'GET', data: { search: currentSearchTerm }, dataType: 'json' })
                        .then(function (data) {
                            if (data.results) {
                                callback.call(this, filterTree(data.results, true));
                                updateSearchStats(data.results.length);
                                setTimeout(() => { tree.jstree(true).open_all(); $('#file-tree a').removeAttr('href'); }, 100);
                            } else { callback.call(this, []); updateSearchStats(0); }
                        })
                        .catch(function (e) { if (e !== '未授权' && e.status !== 401) { callback.call(this, []); updateSearchStats(0); } });
                } else {
                    authAjax({ url: BASE_URL + '/api/files?list=true', method: 'GET', dataType: 'json' })
                        .then(function (data) {
                            callback.call(this, filterTree(data, true));
                            $('#search-stats').text('');
                            setTimeout(() => $('#file-tree a').removeAttr('href'), 100);
                        })
                        .catch(function (e) { if (e !== '未授权' && e.status !== 401) callback.call(this, []); });
                }
            },
            'themes': { 'responsive': true, 'dots': false, 'icons': true },
            'check_callback': true,
            'force_text': true
        },
        'plugins': ['types'],
        'types': {
            'default': { 'icon': 'jstree-folder' },
            'file': { 'icon': 'jstree-file' }
        }
    });

    $('#file-tree').on('ready.jstree after_open.jstree after_close.jstree', function () {
        setTimeout(() => $('#file-tree a').removeAttr('href'), 50);
    });

    // ====== 文件选择 ======
    tree.on('select_node.jstree', function (e, data) {
        const node = data.node;
        if (node.type === 'file') {
            loadFileForEditing(node);
        } else if (node.path) {
            loadFileByPath(node.path, node.text);
        } else {
            const filePath = buildFilePath(node);
            updateBreadcrumb(filePath, true);
            $('#vditor-container').html('<div class="empty-state"><div class="empty-state-icon">📁</div><h3>' + node.text + '</h3><p>选择文件夹内的文件进行编辑</p></div>');
            destroyActiveEditor();
            clearSaveInterval(); currentFile = null; isDirty = false;
        }
    });

    // ====== 文件操作 ======
    function loadFileForEditing(node) {
        if (!checkAuth()) return;
        if (isDirty && currentFile && hasActiveEditor()) {
            if (confirm('当前文件有未保存的更改，是否先保存？')) {
                saveCurrentFile().then(() => loadNewFile(node)).catch(() => loadNewFile(node));
            } else { loadNewFile(node); }
        } else { loadNewFile(node); }
    }

    function loadFileByPath(filePath, fileName) {
        if (!checkAuth()) return;
        if (isDirty && currentFile && hasActiveEditor()) {
            if (confirm('当前文件有未保存的更改，是否先保存？')) {
                saveCurrentFile().then(() => loadNewFileByPath(filePath, fileName)).catch(() => loadNewFileByPath(filePath, fileName));
            } else { loadNewFileByPath(filePath, fileName); }
        } else { loadNewFileByPath(filePath, fileName); }
    }

    function loadNewFile(node) {
        const filePath = buildFilePath(node);
        if (!filePath) { $('#vditor-container').html('<div class="error-box">无法构建文件路径</div>'); return; }
        updateBreadcrumb(filePath, false);
        $('#vditor-container').html('<div class="loading">加载中...</div>');
        $('#status-left').text('加载中...');
        authAjax({ url: BASE_URL + `/api/files/${filePath}`, method: 'GET', dataType: 'text' })
            .then(function (content) {
                currentFile = { id: node.id, name: node.text, path: filePath };
                const savedPosition = getFileScrollPosition(filePath);
                initEditor(content, savedPosition, filePath);
                $('#status-left').text('就绪');
                isDirty = false;
            })
            .catch(function (e) {
                if (e.status !== 401) {
                    $('#vditor-container').html('<div class="error-box">加载失败: ' + (e.responseText || '未知错误') + '</div>');
                    $('#status-left').text('加载失败');
                }
            });
    }

    function loadNewFileByPath(filePath, fileName) {
        updateBreadcrumb(filePath, false);
        $('#vditor-container').html('<div class="loading">加载中...</div>');
        $('#status-left').text('加载中...');
        authAjax({ url: BASE_URL + `/api/files/${filePath}`, method: 'GET', dataType: 'text' })
            .then(function (content) {
                currentFile = { id: 'search-' + filePath, name: fileName, path: filePath };
                const savedPosition = getFileScrollPosition(filePath);
                initEditor(content, savedPosition, filePath);
                $('#status-left').text('就绪');
                isDirty = false;
            })
            .catch(function (e) {
                if (e.status !== 401) {
                    $('#vditor-container').html('<div class="error-box">加载失败: ' + (e.responseText || '未知错误') + '</div>');
                    $('#status-left').text('加载失败');
                }
            });
    }

    function updateBreadcrumb(filePath, isDir) {
        const parts = filePath.split('/');
        let html = '';
        parts.forEach((p, i) => {
            if (i > 0) html += '<span class="sep">/</span>';
            html += `<span${i === parts.length - 1 ? ' class="current"' : ''}>${p}</span>`;
        });
        $('#breadcrumb').html(html);
    }

    function buildFilePath(node) {
        if (node.id === 'root') return '';
        let parts = [];
        let cur = node;
        while (cur && cur.id !== '#' && cur.id !== 'root') {
            if (cur.text && cur.text !== 'root') parts.unshift(cur.text);
            if (cur.parent && cur.parent !== '#') cur = tree.jstree('get_node', cur.parent);
            else break;
        }
        return parts.length > 0 ? parts.join('/') : node.text;
    }

    // ====== 滚动位置持久化 ======
    function saveFileScrollPosition(filePath) {
        if ((!vditor && !aceEditor) || !filePath) return;
        try {
            const d = { previewScrollTop: 0, irScrollTop: 0, svScrollTop: 0, wysiwygScrollTop: 0, timestamp: Date.now() };
            if (vditor?.vditor?.preview?.element) d.previewScrollTop = vditor.vditor.preview.element.scrollTop;
            if (vditor?.vditor?.ir?.element) d.irScrollTop = vditor.vditor.ir.element.scrollTop;
            if (vditor?.vditor?.sv?.element) d.svScrollTop = vditor.vditor.sv.element.scrollTop;
            if (vditor?.vditor?.wysiwyg?.element) d.wysiwygScrollTop = vditor.vditor.wysiwyg.element.scrollTop;
            if (aceEditor) {
                d.aceScrollTop = aceEditor.session.getScrollTop();
                d.aceScrollLeft = aceEditor.session.getScrollLeft();
                d.aceCursor = aceEditor.getCursorPosition();
            }
            localStorage.setItem(`file_scroll_position_${filePath}`, JSON.stringify(d));
        } catch (e) { /* ignore */ }
    }

    function getFileScrollPosition(filePath) {
        if (!filePath) return null;
        try { const s = localStorage.getItem(`file_scroll_position_${filePath}`); return s ? JSON.parse(s) : null; }
        catch (e) { return null; }
    }

    // ====== 编辑器选择：Markdown 使用 Vditor，其他文本使用 Ace ======
    function isMarkdownFile(filePath) {
        return /\.(md|markdown|mkd|mdown|mkdn|mdwn|mdtxt|mdtext|rmd)$/i.test(filePath || '');
    }

    function hasActiveEditor() {
        return Boolean(vditor || aceEditor);
    }

    function destroyActiveEditor() {
        if (vditor) {
            vditor.destroy();
            vditor = null;
            window.vditorInstance = null;
        }
        if (aceEditor) {
            aceEditor.destroy();
            aceEditor = null;
            window.aceEditorInstance = null;
        }
    }

    function markDirty() {
        if (!isDirty) {
            isDirty = true;
            $('#save-indicator').addClass('dirty').removeClass('saved').text('● 未保存');
        }
    }

    function initEditor(content, savedPosition = null, filePath = '') {
        destroyActiveEditor();
        if (!isMarkdownFile(filePath)) {
            initTextEditor(content, savedPosition, filePath);
            return;
        }
        $('#vditor-container').html('<div id="vditor"></div>');

        const isMobile = window.innerWidth <= 576;
        // 获取当前主题对应的 Vditor 主题名
        const resolvedTheme = resolveTheme(getStoredTheme());

        vditor = new Vditor('vditor', {
            cdn: '/assets/vendor/vditor',
            height: isMobile ? (window.innerHeight - 40) + 'px' : '100%',
            value: content,
            cache: { enable: false },
            placeholder: '在此编辑文件内容...',
            toolbar: [
                "emoji", "headings", "bold", "italic", "strike", "link", "|",
                "list", "ordered-list", "check", "outdent", "indent", "|",
                "quote", "line", "code", "inline-code", "insert-before", "insert-after", "|",
                "upload", "record", "table", "|",
                "undo", "redo", "|",
                "edit-mode", "content-theme", "code-theme", "export",
                { name: "more", toolbar: ["fullscreen", "both", "preview", "info", "help"] }
            ],
            toolbarConfig: { pin: true },
            theme: {
                current: getVditorThemeName(resolvedTheme),
                path: '/assets/vendor/vditor/dist/css/content-theme',
            },
            outline: { enable: true, position: 'right' },
            input: function () {
                markDirty();
                if (currentFile?.path) saveFileScrollPosition(currentFile.path);
            },
            focus: function () { startAutoSave(); },
            blur: function () {
                clearSaveInterval();
                if (currentFile?.path) saveFileScrollPosition(currentFile.path);
            },
            after: function () {
                startAutoSave();
                window.vditorInstance = vditor;
                // 确保 Vditor 主题与当前主题一致
                if (savedPosition && vditor) {
                    setTimeout(() => {
                        try {
                            if (vditor.vditor?.preview?.element) vditor.vditor.preview.element.scrollTop = savedPosition.previewScrollTop || 0;
                            if (vditor.vditor?.ir?.element) vditor.vditor.ir.element.scrollTop = savedPosition.irScrollTop || 0;
                            if (vditor.vditor?.sv?.element) vditor.vditor.sv.element.scrollTop = savedPosition.svScrollTop || 0;
                            if (vditor.vditor?.wysiwyg?.element) vditor.vditor.wysiwyg.element.scrollTop = savedPosition.wysiwygScrollTop || 0;
                        } catch (e) { /* ignore */ }
                    }, 100);
                }
                if (window.innerWidth <= 576) adjustEditorHeight();
            },
            resize: function () {
                if (currentFile?.path) saveFileScrollPosition(currentFile.path);
                if (window.innerWidth <= 576) adjustEditorHeight();
            }
        });
        isDirty = false;
    }

    function initTextEditor(content, savedPosition, filePath) {
        $('#vditor-container').html('<div id="ace-editor"></div>');
        aceEditor = ace.edit('ace-editor');
        window.aceEditorInstance = aceEditor;

        const resolvedTheme = resolveTheme(getStoredTheme());
        aceEditor.setTheme(resolvedTheme === 'dark' ? 'ace/theme/tomorrow_night' : 'ace/theme/textmate');
        aceEditor.session.setValue(content || '');
        aceEditor.session.setUseWorker(false);
        aceEditor.session.setUseWrapMode(true);
        aceEditor.setOptions({
            fontSize: '14px',
            showPrintMargin: false,
            tabSize: 4,
            useSoftTabs: true,
            scrollPastEnd: 0.25
        });

        try {
            const modelist = ace.require('ace/ext/modelist');
            aceEditor.session.setMode(modelist.getModeForPath(filePath).mode);
        } catch (e) {
            aceEditor.session.setMode('ace/mode/text');
        }

        aceEditor.session.getUndoManager().reset();
        aceEditor.session.on('change', function () {
            markDirty();
        });
        aceEditor.on('focus', startAutoSave);
        aceEditor.on('blur', function () {
            clearSaveInterval();
            if (currentFile?.path) saveFileScrollPosition(currentFile.path);
        });

        if (savedPosition) {
            aceEditor.session.setScrollTop(savedPosition.aceScrollTop || 0);
            aceEditor.session.setScrollLeft(savedPosition.aceScrollLeft || 0);
            if (savedPosition.aceCursor) aceEditor.moveCursorToPosition(savedPosition.aceCursor);
        }
        aceEditor.clearSelection();
        aceEditor.resize();
        startAutoSave();
        isDirty = false;
    }

    function adjustEditorHeight() {
        if (vditor && window.innerWidth <= 576) {
            const c = document.getElementById('vditor-container');
            if (c) { c.style.height = (window.innerHeight - 40) + 'px'; setTimeout(() => vditor.resize(), 100); }
        }
    }

    // ====== 保存 ======
    function startAutoSave() {
        clearSaveInterval();
        saveInterval = setInterval(() => { if (isDirty && currentFile && hasActiveEditor()) saveCurrentFile(); }, 3000);
    }

    function clearSaveInterval() { if (saveInterval) { clearInterval(saveInterval); saveInterval = null; } }

    function saveCurrentFile() {
        return new Promise((resolve, reject) => {
            if (!currentFile || !hasActiveEditor()) { reject('没有文件可保存'); return; }
            const content = vditor ? vditor.getValue() : aceEditor.getValue();
            $('#save-indicator').text('保存中...').removeClass('dirty saved');
            authAjax({ url: BASE_URL + `/api/files/${currentFile.path}`, method: 'POST', contentType: 'text/plain', data: content })
                .then(function () {
                    isDirty = false;
                    lastSaveTime = new Date();
                    $('#save-indicator').addClass('saved').removeClass('dirty').text('✓ 已保存 ' + lastSaveTime.toLocaleTimeString());
                    resolve();
                })
                .catch(function (e) {
                    if (e.status !== 401) $('#save-indicator').text('保存失败').addClass('dirty');
                    reject(e);
                });
        });
    }

    // Ctrl+S
    $(document).on('keydown', function (e) {
        if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); if (currentFile && hasActiveEditor()) saveCurrentFile(); }
    });

    // ====== 拖拽调整 ======
    function initResizeHandler() {
        const handle = document.getElementById('resize-handle');
        const sidebar = document.getElementById('sidebar');
        let startX, startWidth;

        handle.addEventListener('mousedown', function (e) {
            isResizing = true; startX = e.clientX;
            startWidth = parseInt(getComputedStyle(sidebar).width, 10);
            handle.classList.add('active');
            document.addEventListener('mousemove', onMove);
            document.addEventListener('mouseup', onUp);
            e.preventDefault();
        });

        function onMove(e) {
            if (!isResizing) return;
            const w = startWidth + e.clientX - startX;
            if (w >= 200 && w <= 500) {
                sidebar.style.width = w + 'px';
                if (vditor) setTimeout(() => vditor.resize(), 50);
                if (aceEditor) setTimeout(() => aceEditor.resize(), 50);
            }
        }

        function onUp() {
            isResizing = false; handle.classList.remove('active');
            document.removeEventListener('mousemove', onMove);
            document.removeEventListener('mouseup', onUp);
            const w = parseInt(sidebar.style.width, 10);
            if (w) localStorage.setItem('sidebar-width', w);
        }

        const savedW = localStorage.getItem('sidebar-width');
        if (savedW) sidebar.style.width = savedW + 'px';
    }

    // ====== 窗口事件 ======
    window.addEventListener('beforeunload', function (e) {
        if (currentFile?.path) saveFileScrollPosition(currentFile.path);
        if (isDirty) { e.preventDefault(); e.returnValue = '有未保存的更改'; return e.returnValue; }
    });

    $(window).on('resize', function () {
        if (vditor) setTimeout(() => vditor.resize(), 100);
        if (aceEditor) setTimeout(() => aceEditor.resize(), 100);
        if (window.innerWidth <= 576) adjustEditorHeight();
    });

    // ====== 移动端侧边栏 ======
    (function () {
        const toggle = document.getElementById('mobile-toggle');
        const sidebar = document.getElementById('sidebar');
        const backdrop = document.getElementById('overlay-backdrop');
        if (!toggle) return;

        toggle.addEventListener('click', () => { sidebar.classList.add('sidebar-open'); backdrop.classList.add('visible'); });
        backdrop.addEventListener('click', () => { sidebar.classList.remove('sidebar-open'); backdrop.classList.remove('visible'); });

        $('#file-tree').on('click', 'a', function () {
            if (window.innerWidth <= 576) {
                const nodeId = $(this).closest('li').attr('id');
                const node = tree.jstree('get_node', nodeId);
                if (node?.type === 'file') {
                    sidebar.classList.remove('sidebar-open');
                    backdrop.classList.remove('visible');
                    setTimeout(() => {
                        if (vditor) vditor.resize();
                        if (aceEditor) aceEditor.resize();
                    }, 300);
                }
            }
        });
    })();

    // ====== 新建 / 删除功能 ======
    let contextNode = null;

    window.closeModal = function (id) { document.getElementById(id).classList.remove('active'); };
    function openModal(id) { document.getElementById(id).classList.add('active'); }

    function getSelectedDirPath() {
        const sel = tree.jstree('get_selected', true);
        if (!sel.length) return '';
        const node = sel[0];
        if (node.type === 'file') {
            const parent = tree.jstree('get_node', node.parent);
            return (parent && parent.id !== '#' && parent.id !== 'root') ? buildFilePath(parent) : '';
        }
        return buildFilePath(node);
    }

    $('#btn-new-file').on('click', function () {
        const dir = getSelectedDirPath();
        const input = document.getElementById('input-new-file');
        input.value = ''; input.dataset.dir = dir;
        openModal('modal-new-file');
        setTimeout(() => input.focus(), 100);
    });

    $('#btn-new-folder').on('click', function () {
        const dir = getSelectedDirPath();
        const input = document.getElementById('input-new-folder');
        input.value = ''; input.dataset.dir = dir;
        openModal('modal-new-folder');
        setTimeout(() => input.focus(), 100);
    });

    $('#btn-delete').on('click', function () {
        const sel = tree.jstree('get_selected', true);
        if (!sel.length) { alert('请先选择要删除的文件或文件夹'); return; }
        confirmAndDelete(sel[0]);
    });

    $('#btn-confirm-new-file').on('click', function () {
        const input = document.getElementById('input-new-file');
        const name = input.value.trim();
        if (!name) { input.style.borderColor = 'var(--semantic-error)'; return; }
        input.style.borderColor = '';
        const dir = input.dataset.dir || '';
        const fullPath = dir ? dir + '/' + name : name;
        authAjax({ url: BASE_URL + '/api/create-file', method: 'POST', contentType: 'application/json', data: JSON.stringify({ path: fullPath, content: '' }) })
            .then(() => { closeModal('modal-new-file'); tree.jstree(true).refresh(); $('#status-left').text('文件创建成功'); })
            .catch((e) => { if (e.status !== 401) { const m = e.responseJSON?.message || '创建失败'; alert(m); } });
    });

    $('#btn-confirm-new-folder').on('click', function () {
        const input = document.getElementById('input-new-folder');
        const name = input.value.trim();
        if (!name) { input.style.borderColor = 'var(--semantic-error)'; return; }
        input.style.borderColor = '';
        const dir = input.dataset.dir || '';
        const fullPath = dir ? dir + '/' + name : name;
        authAjax({ url: BASE_URL + '/api/mkdir', method: 'POST', contentType: 'application/json', data: JSON.stringify({ path: fullPath }) })
            .then(() => { closeModal('modal-new-folder'); tree.jstree(true).refresh(); $('#status-left').text('文件夹创建成功'); })
            .catch((e) => { if (e.status !== 401) { const m = e.responseJSON?.message || '创建失败'; alert(m); } });
    });

    function confirmAndDelete(node) {
        const filePath = buildFilePath(node);
        if (!filePath) { alert('无法删除根目录'); return; }
        const type = node.type === 'file' ? '文件' : '文件夹';
        const suffix = node.type !== 'file' ? ' 文件夹内所有内容将被删除。' : '';
        document.getElementById('delete-message').textContent = `确定要删除${type} "${node.text}" 吗？${suffix}`;
        document.getElementById('btn-confirm-delete').dataset.path = filePath;
        openModal('modal-delete');
    }

    $('#btn-confirm-delete').on('click', function () {
        const filePath = this.dataset.path;
        if (!filePath) return;
        authAjax({ url: BASE_URL + '/api/files/' + filePath, method: 'DELETE' })
            .then(() => {
                closeModal('modal-delete');
                if (currentFile && (currentFile.path === filePath || filePath.startsWith(currentFile.path + '/'))) {
                    destroyActiveEditor();
                    currentFile = null; isDirty = false;
                    $('#vditor-container').html('<div class="empty-state"><div class="empty-state-icon">📝</div><h3>选择文件开始编辑</h3><p>从左侧选择一个文件，或创建新文件</p></div>');
                    $('#breadcrumb').html('<span class="current">EasyNotes</span>');
                    $('#save-indicator').text('');
                }
                tree.jstree(true).refresh();
                $('#status-left').text('删除成功');
            })
            .catch((e) => { if (e.status !== 401) { const m = e.responseJSON?.message || '删除失败'; alert(m); } });
    });

    // 回车确认
    $('#input-new-file').on('keydown', (e) => { if (e.key === 'Enter') $('#btn-confirm-new-file').click(); });
    $('#input-new-folder').on('keydown', (e) => { if (e.key === 'Enter') $('#btn-confirm-new-folder').click(); });

    // 弹窗外部关闭
    $('.modal-overlay').on('click', function (e) { if (e.target === this) closeModal(this.id); });

    // ESC
    $(document).on('keydown', function (e) {
        if (e.key === 'Escape') { $('.modal-overlay.active').each(function () { closeModal(this.id); }); $('#context-menu').hide(); }
    });

    // ====== 右键菜单 ======
    $('#file-tree').on('contextmenu', '.jstree-anchor', function (e) {
        e.preventDefault(); e.stopPropagation();
        const nodeId = $(this).closest('li').attr('id');
        const node = tree.jstree('get_node', nodeId);
        if (!node) return;
        tree.jstree('select_node', nodeId);
        contextNode = node;

        // 根据节点类型显隐菜单项
        const isFolder = node.type !== 'file';
        document.getElementById('ctx-folder-items').style.display = isFolder ? 'block' : 'none';

        const menu = document.getElementById('context-menu');
        menu.style.display = 'block';
        let x = e.clientX, y = e.clientY;
        if (x + 180 > window.innerWidth) x = window.innerWidth - 190;
        if (y + 120 > window.innerHeight) y = window.innerHeight - 130;
        menu.style.left = x + 'px'; menu.style.top = y + 'px';
    });

    $('#context-menu').on('click', '.ctx-item', function () {
        const action = $(this).data('action');
        $('#context-menu').hide();
        if (!contextNode) return;
        if (action === 'new-file') {
            // 在右键文件夹的子层创建
            const dir = contextNode.type !== 'file' ? buildFilePath(contextNode) : '';
            const input = document.getElementById('input-new-file');
            input.value = ''; input.dataset.dir = dir;
            openModal('modal-new-file'); setTimeout(() => input.focus(), 100);
        } else if (action === 'new-folder') {
            // 在右键文件夹的子层创建
            const dir = contextNode.type !== 'file' ? buildFilePath(contextNode) : '';
            const input = document.getElementById('input-new-folder');
            input.value = ''; input.dataset.dir = dir;
            openModal('modal-new-folder'); setTimeout(() => input.focus(), 100);
        } else if (action === 'delete') {
            confirmAndDelete(contextNode);
        }
    });

    $(document).on('click', () => $('#context-menu').hide());

    // 初始检查
    if (!checkAuth()) handleUnauthorized();
});
