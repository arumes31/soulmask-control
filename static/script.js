let ws;
let reconnectInterval = 3000;

async function logout() {
    await fetch('/logout', { method: 'POST' });
    window.location.href = '/login';
}

async function updateStatus() {
    try {
        const response = await fetch('/api/status');
        if (response.status === 401) {
            window.location.href = '/login';
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

        document.getElementById('container-id').textContent = data.id || '--------';
        document.getElementById('container-image').textContent = data.image || 'N/A';
    } catch (e) {
        console.error('Status fetch error', e);
    }
}

function confirmAction(type) {
    const modal = document.getElementById('modal-container');
    const title = document.getElementById('modal-title');
    const desc = document.getElementById('modal-desc');
    const icon = document.getElementById('modal-icon');
    const confirmBtn = document.getElementById('modal-confirm-btn');
    
    let colorClass = "";
    let iconSvg = "";
    
    switch(type) {
        case 'start':
            title.textContent = "Start Instance";
            desc.textContent = "Initialize the Soulmask container and bring systems online.";
            colorClass = "bg-green-600";
            iconSvg = `<svg class="w-8 h-8 text-green-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd" /></svg>`;
            break;
        case 'restart':
            title.textContent = "Restart Instance";
            desc.textContent = "Recycle the active process. This will temporarily drop all connections.";
            colorClass = "bg-blue-600";
            iconSvg = `<svg class="w-8 h-8 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>`;
            break;
        case 'stop':
            title.textContent = "Stop Instance";
            desc.textContent = "Terminate the container process. All active sessions will be killed.";
            colorClass = "bg-red-600";
            iconSvg = `<svg class="w-8 h-8 text-red-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd" /></svg>`;
            break;
    }
    
    icon.innerHTML = iconSvg;
    confirmBtn.className = `flex-1 px-4 py-3 rounded-xl text-xs font-bold transition-all shadow-lg active:scale-95 uppercase tracking-widest text-white ${colorClass}`;
    confirmBtn.onclick = () => executeAction(type);
    
    modal.classList.remove('hidden');
}

function closeModal() {
    document.getElementById('modal-container').classList.add('hidden');
}

async function executeAction(name) {
    closeModal();
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
        div.className = 'text-green-500 font-bold border-l-2 border-green-500 pl-2 my-2';
        div.textContent = `[${new Date().toLocaleTimeString()}] UPLINK_ESTABLISHED: STREAM_CONNECTED`;
        terminal.appendChild(div);
        connStatus.innerHTML = '<div class="w-2 h-2 rounded-full bg-green-500 animate-pulse"></div><span class="text-xs font-medium text-gray-300 uppercase tracking-tighter">Live Stream Active</span>';
    };

    ws.onmessage = (event) => {
        const line = document.createElement('div');
        line.className = 'hover:bg-white/5 transition-colors border-l border-transparent hover:border-gray-700 pl-2 py-0.5';
        
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

// Initialization
updateStatus();
setInterval(updateStatus, 5000);
connectLogs();
