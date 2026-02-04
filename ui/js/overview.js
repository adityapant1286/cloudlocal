// system overview
let eventSource = null;

export async function fetchOverviewData() {
  try {
    const res = await fetch('/dashboard/api/status');
    const data = await res.json();

    // 1. Update Sidebar Status
    const serviceList = document.getElementById('service-list');
    serviceList.innerHTML = data.services.map(s => {
      const menuItem = document.getElementById(`menu-item-${s.id}`.toLowerCase())
      if (menuItem) {
        menuItem.classList.remove('hidden');
      }
      return `
        <div class="flex items-center justify-between group">
          <span class="text-sm font-medium ${s.healthy ? 'text-slate-200' : 'text-gray-600'}">${s.name}</span>
          <div class="flex items-center gap-2">
            <div class="w-2 h-2 rounded-full ${s.healthy ? 'bg-green-500 shadow-[0_0_8px_#22c55e]' : 'bg-red-500 status-pulse'}"></div>
          </div>
        </div>
      `}).join('');

    // 2. Update Storage Grid
    /*
        const storageGrid = document.getElementById('storage-grid');
        storageGrid.innerHTML = Object.entries(data.storage).map(([svc, size]) => `
                        <div class="bg-stone-900 border border-stone-800 p-4 rounded-2xl hover:border-stone-700 transition-colors">
                            <div class="text-xs font-bold text-gray-400 tracking-tighter">${svc}</div>
                            <div class="text-2xl text-white mt-2">${size}</div>
                        </div>
                    `).join('');
    */

    document.getElementById('vol-path').innerText = data.volume_path;

    /*
        if (data.buckets !== undefined) {
          const s3Container = document.getElementById('s3-actions');
          s3Container.innerHTML = data.buckets.map(b => `
            <div class="flex items-center justify-between p-2 hover:bg-stone-800 rounded-lg transition-colors">
                <span class="text-sm font-mono">${b}</span>
                <button onclick="triggerAction('clear_s3', 's3', '${b}')" class="text-xs bg-red-900/40 text-red-400 px-3 py-1 rounded-md border border-red-500/30 hover:bg-red-500 hover:text-white transition-all">Clear</button>
            </div>
          `).join('');
        }
    */

  } catch (err) {
    console.error("Dashboard sync error:", err);
  }

}

export function downloadLogs() {
  const service = document.getElementById('log-selector').value;
  // Simply navigate the window to the download URL
  window.location.href = `/dashboard/api/logs/download?service=${service}`;

}

export function switchLog() {
  const service = document.getElementById('log-selector').value;
  const consoleEl = document.getElementById('log-console');

  // Close existing stream
  if (eventSource) {
    eventSource.close();
  }
  consoleEl.innerHTML = `<div class="text-blue-400 font-bold pb-1">[SYSTEM] Switched to ${service} logs...</div>`;

  // Start new SSE connection
  eventSource = new EventSource(`/dashboard/api/logs?service=${service}`);

  eventSource.onmessage = function (event) {
    const line = document.createElement('div');
    // line.className = "border-b border-stone-900 py-1";
    line.className = "pb-1";
    line.textContent = event.data;
    consoleEl.appendChild(line);

    // Auto-scroll to bottom
    consoleEl.scrollTop = consoleEl.scrollHeight;
  };

  eventSource.onerror = function () {
    console.error("Log stream lost. Reconnecting...");
  };
}
