let ws;
let reconnectInterval = 3000;

function showDashboard() {
    document.body.className = "bg-gray-900 min-h-screen text-gray-100 font-sans";
    document.getElementById('login-container').classList.add('hidden');
    document.getElementById('dashboard').classList.remove('hidden');
    updateStatus();
    setInterval(updateStatus, 5000);
    connectLogs();
}

function showLogin() {
    document.body.className = "bg-gray-900 min-h-screen text-gray-100 font-sans";
    document.getElementById('login-container').classList.remove('hidden');
    document.getElementById('dashboard').classList.add('hidden');
    if (ws) ws.close();
}

async function login() {
    const passwordEl = document.getElementById('password');
    const loginBox = document.getElementById('login-box');
    const password = passwordEl.value;
    
    const response = await fetch('/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password })
    });

    if (response.ok) {
        showDashboard();
    } else {
        loginBox.classList.add('shake');
        setTimeout(() => loginBox.classList.remove('shake'), 500);
        passwordEl.value = '';
        passwordEl.placeholder = "INVALID CODE";
        setTimeout(() => {
            passwordEl.placeholder = "ENTER ACCESS CODE";
        }, 2000);
    }
}

document.getElementById('password').addEventListener('keypress', function (e) {
    if (e.key === 'Enter') {
        login();
    }
});

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
        const statusBadge = document.getElementById('container-status-badge');
        
        statusBadge.textContent = data.status || 'Unknown';
        
        const btnStart = document.getElementById('btn-start');
        const btnStop = document.getElementById('btn-stop');
        const btnRestart = document.getElementById('btn-restart');

        if (data.status === 'running') {
            statusBadge.className = 'px-2 py-1 rounded-md text-[10px] font-black uppercase tracking-tighter bg-green-500/20 text-green-500 border border-green-500/30';
            btnStart.disabled = true;
            btnStart.classList.add('opacity-50', 'cursor-not-allowed');
            btnStop.disabled = false;
            btnStop.classList.remove('opacity-50', 'cursor-not-allowed');
        } else {
            statusBadge.className = 'px-2 py-1 rounded-md text-[10px] font-black uppercase tracking-tighter bg-red-500/20 text-red-500 border border-red-500/30';
            btnStart.disabled = false;
            btnStart.classList.remove('opacity-50', 'cursor-not-allowed');
            btnStop.disabled = true;
            btnStop.classList.add('opacity-50', 'cursor-not-allowed');
        }

        document.getElementById('container-image').textContent = data.image || 'N/A';
        document.getElementById('container-id').textContent = data.id || '--------';
    } catch (e) {
        console.error('Status fetch error', e);
    }
}

async function action(name) {
    try {
        const response = await fetch(`/api/action/${name}`, { method: 'POST' });
        if (!response.ok) {
            const err = await response.text();
            alert('Action failed: ' + err);
        }
        updateStatus();
    } catch (e) {
        alert('Action failed: ' + e);
    }
}

function connectLogs() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/api/logs`);
    const terminal = document.getElementById('terminal');
    const connStatus = document.getElementById('connection-status');

    ws.onopen = () => {
        const div = document.createElement('div');
        div.className = 'text-blue-500 font-bold border-l-2 border-blue-500 pl-2 my-2';
        div.textContent = `[${new Date().toLocaleTimeString()}] UPLINK_ESTABLISHED: STREAM_CONNECTED`;
        terminal.appendChild(div);
        connStatus.innerHTML = '<div class="w-2 h-2 rounded-full bg-green-500 animate-pulse"></div><span class="text-xs font-medium text-gray-300 uppercase tracking-tighter">Live Stream Active</span>';
    };

    ws.onmessage = (event) => {
        const line = document.createElement('div');
        line.className = 'hover:bg-gray-900/50 transition-colors border-l border-transparent hover:border-gray-800 pl-2 py-0.5';
        
        const timeSpan = document.createElement('span');
        timeSpan.className = 'text-gray-600 mr-3 text-[10px] select-none';
        timeSpan.textContent = new Date().toLocaleTimeString([], {hour12: false});
        
        const contentSpan = document.createElement('span');
        contentSpan.textContent = event.data;
        
        line.appendChild(timeSpan);
        line.appendChild(contentSpan);
        terminal.appendChild(line);
        terminal.scrollTop = terminal.scrollHeight;
        
        if (terminal.childNodes.length > 2000) {
            terminal.removeChild(terminal.firstChild);
        }
    };

    ws.onclose = () => {
        connStatus.innerHTML = '<div class="w-2 h-2 rounded-full bg-red-500"></div><span class="text-xs font-medium text-gray-500 uppercase tracking-tighter">Stream Offline</span>';
        setTimeout(connectLogs, reconnectInterval);
    };
}

function clearLogs() {
    document.getElementById('terminal').innerHTML = '';
}

fetch('/api/status').then(res => {
    if (res.ok) showDashboard();
    else showLogin();
}).catch(() => showLogin());
