        // 主题切换
        const THEME_KEY = 'easynotes-theme';
        function getStoredTheme() { return localStorage.getItem(THEME_KEY) || 'auto'; }
        function resolveTheme(t) {
            if (t === 'auto') return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
            return t;
        }
        function updateThemeIcon() {
            const btn = document.getElementById('theme-toggle');
            const stored = getStoredTheme();
            if (stored === 'auto') btn.textContent = '🖥';
            else if (resolveTheme(stored) === 'dark') btn.textContent = '🌙';
            else btn.textContent = '☀';
        }
        document.getElementById('theme-toggle').addEventListener('click', function() {
            const current = getStoredTheme();
            const next = current === 'light' ? 'dark' : current === 'dark' ? 'auto' : 'light';
            localStorage.setItem(THEME_KEY, next);
            document.documentElement.setAttribute('data-theme', resolveTheme(next));
            updateThemeIcon();
        });
        updateThemeIcon();

        const form = document.getElementById('login-form');
        const btn = document.getElementById('login-btn');
        const errMsg = document.getElementById('error-msg');

        form.addEventListener('submit', async function(e) {
            e.preventDefault();
            errMsg.classList.remove('visible');

            const username = document.getElementById('username').value.trim();
            const password = document.getElementById('password').value;

            if (!username || !password) {
                showError('请输入用户名和密码');
                return;
            }

            btn.disabled = true;
            btn.textContent = '登录中...';

            try {
                const resp = await fetch('/api/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ username, password })
                });

                const data = await resp.json();

                if (data.success) {
                    localStorage.setItem('token', data.token);
                    const redirect = new URLSearchParams(window.location.search).get('redirect') || '/editor';
                    window.location.href = redirect;
                } else {
                    showError(data.message || '登录失败');
                }
            } catch (err) {
                showError('网络错误，请重试');
            } finally {
                btn.disabled = false;
                btn.textContent = '登录';
            }
        });

        function showError(msg) {
            errMsg.textContent = msg;
            errMsg.classList.add('visible');
        }
