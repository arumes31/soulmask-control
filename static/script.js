let ws;
let reconnectInterval = 3000;

function showDashboard() {
    document.getElementById('login-container').classList.add('hidden');
    document.getElementById('dashboard').classList.remove('hidden');
    updateStatus();
    setInterval(updateStatus, 5000);
    connectLogs();
}

function showLogin() {
    document.getElementById('login-container').classList.remove('hidden');
    document.getElementById('dashboard').classList.add('hidden');
    if (ws) ws.close();
}

async function login() {
    const password = document.getElementById('password').value;
    const response = await fetch('/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password })
    });

    if (response.ok) {
        showDashboard();
    } else {
        alert('Login failed');
    }
}

async function logout() {
    await fetch('/logout', { method: 'POST' });
    showLogin();
}

async function updateStatus() {
    try {
        const response = await fetch('/api/status');
        if (response.status === 401) {
            showLogin();
            return;
        }
        const data = await response.json();
        const statusEl = document.getElementById('container-status');
        statusEl.textContent = data.status || 'Unknown';
        
        // Color coding
        if (data.status === 'running') statusEl.className = 'font-bold text-green-600';
        else if (data.status === 'exited') statusEl.className = 'font-bold text-red-600';
        else statusEl.className = 'font-bold text-yellow-600';

        document.getElementById('container-image').textContent = data.image || '-';
        document.getElementById('container-id').textContent = data.id || '-';
    } catch (e) {
        console.error('Status fetch error', e);
    }
}

async function action(name) {
    try {
        const response = await fetch(`/api/action/${name}`, { method: 'POST' });
        if (response.ok) {
            setTimeout(updateStatus, 1000);
        } else {
            const err = await response.text();
            alert('Action failed: ' + err);
        }
    } catch (e) {
        alert('Action failed: ' + e);
    }
}

function connectLogs() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/api/logs`);
    const terminal = document.getElementById('terminal');

    ws.onopen = () => {
        const div = document.createElement('div');
        div.className = 'text-blue-400';
        div.textContent = '*** Connected to log stream ***';
        terminal.appendChild(div);
    };

    ws.onmessage = (event) => {
        const div = document.createElement('div');
        div.textContent = event.data;
        terminal.appendChild(div);
        terminal.scrollTop = terminal.scrollHeight;
        
        // Keep terminal from growing too large
        if (terminal.childNodes.length > 1000) {
            terminal.removeChild(terminal.firstChild);
        }
    };

    ws.onclose = () => {
        const div = document.createElement('div');
        div.className = 'text-red-400';
        div.textContent = '*** Disconnected. Reconnecting... ***';
        terminal.appendChild(div);
        setTimeout(connectLogs, reconnectInterval);
    };

    ws.onerror = (err) => {
        console.error('WebSocket error', err);
        ws.close();
    };
}

function clearLogs() {
    document.getElementById('terminal').innerHTML = '';
}

// Initial check
fetch('/api/status').then(res => {
    if (res.ok) showDashboard();
}).catch(() => {});
