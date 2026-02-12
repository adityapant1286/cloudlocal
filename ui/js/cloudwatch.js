import {styleSelectedElement, toISODateFormat} from "./utils.js";

// Cloudwatch
let cwCurrentLogGroup = "";
let cwCurrentLogStream = "";

export async function cwLoadLogGroups() {
  const res = await fetch('/dashboard/api/logs/groups');
  const data = await res.json();
  const container = document.getElementById('cw-group-list');

  if (data && data.logGroups) {
    container.innerHTML = data.logGroups.map(g => `
        <button onclick="selectCwLogGroup(this, '${g.logGroupName}')" class="cls-cw-log-group w-full text-left px-3 py-2 rounded-md text-xs transition-all hover:bg-neutral-800 text-slate-300 hover:text-white truncate">
            ${g.logGroupName}
        </button>
    `).join('');
  }
}

export async function selectCwLogGroup(e, groupName) {
  cwCurrentLogGroup = groupName;
  document.getElementById('cw-streams-panel').classList.remove('hidden');
  document.getElementById('cw-events-panel').classList.add('hidden');

  styleSelectedElement(e, 'button.cls-cw-log-group');

  const res = await fetch(`/dashboard/api/logs/streams?group=${encodeURIComponent(groupName)}`);
  const data = await res.json();
  const container = document.getElementById('cw-stream-list');

  container.innerHTML = data.logStreams.map(s => `
        <button onclick="selectCwLogStream(this, '${s.logStreamName}')" class="cls-cw-log-group-stream w-full text-left px-3 py-2 rounded-md text-[11px] transition-all hover:bg-neutral-800 text-slate-300 hover:text-orange-400 truncate font-mono">
            ${s.logStreamName}
        </button>
    `).join('');
}

export async function selectCwLogStream(e, streamName) {
  cwCurrentLogStream = streamName;
  document.getElementById('cw-events-panel').classList.remove('hidden');
  document.getElementById('active-stream-name').innerText = streamName;

  styleSelectedElement(e, 'button.cls-cw-log-group-stream');

  await cwRefreshLogs();
}

export function cwApplyLogFilter(keyword) {
  const term = keyword.toLowerCase();
  const rows = document.querySelectorAll('#cw-event-list > div');

  rows.forEach(row => {
    // We only search the message part, not the timestamp
    const messageText =
        row.querySelector('.log-message-body').innerText.toLowerCase();

    if (messageText.includes(term)) {
      row.classList.remove('hidden');
    } else {
      row.classList.add('hidden');
    }
  });
}

export async function cwRefreshLogs() {
  const container = document.getElementById('cw-event-list');
  const res = await fetch(
      `/dashboard/api/logs/events?group=${encodeURIComponent(cwCurrentLogGroup)}&stream=${encodeURIComponent(cwCurrentLogStream)}`);
  const data = await res.json();

  container.innerHTML = data.events.map(e => {
    const date = toISODateFormat(e.timestamp);
    let level = e.level;

    level = level.replace(/DEBUG/g, '<span class="text-cyan-400 font-bold">DEBUG</span>');
    level = level.replace(/ERROR/g, '<span class="text-red-500 font-bold">ERROR</span>');
    level = level.replace(/INFO/g, '<span class="text-blue-400 font-bold">INFO</span>');
    level = level.replace(/WARN/g, '<span class="text-yellow-500 font-bold">WARN</span>');

    return `
        <div class="pb-1 mb-2 border-b border-neutral-700/70 flex gap-4 group hover:bg-white/5 transition-colors">
            <span class="text-gray-300 shrink-0">${date}</span>
            <span class="log-message-body text-gray-200 break-all whitespace-pre-wrap font-mono">${level} - ${e.message}</span>
        </div>`;
  }).join('');

  // Re-apply any existing filter after refresh
  const currentFilter = document.getElementById('cw-log-filter').value;
  if (currentFilter) {
    cwApplyLogFilter(currentFilter);
  }
  container.scrollTop = container.scrollHeight;
}