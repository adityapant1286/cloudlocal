
// Dynamodb
const DynamoDbParser = {
  // Convert DynamoDB JSON -> Standard JSON
  unmarshall: function(data) {
    if (!data || typeof data !== 'object') return data;
    if (data.S !== undefined) return data.S;
    if (data.N !== undefined) return Number(data.N);
    if (data.BOOL !== undefined) return data.BOOL;
    if (data.NULL !== undefined) return null;
    if (data.M) {
      let obj = {};
      for (let k in data.M) obj[k] = this.unmarshall(data.M[k]);
      return obj;
    }
    if (data.L) return data.L.map(v => this.unmarshall(v));
    return data;
  },

  // Convert Standard JSON -> DynamoDB JSON (for Create/Update)
  marshall: function(data) {
    if (typeof data === 'string') return { S: data };
    if (typeof data === 'number') return { N: data.toString() };
    if (typeof data === 'boolean') return { BOOL: data };
    if (data === null) return { NULL: true };
    if (Array.isArray(data)) return { L: data.map(v => this.marshall(v)) };
    if (typeof data === 'object') {
      let m = {};
      for (let k in data) m[k] = this.marshall(data[k]);
      return { M: m };
    }
    return { S: String(data) };
  }
};

let currentDdbTable = null;
let ddbLastEvaluatedKey = null;
let ddbActiveFilter = null;
let ddbRefreshInterval = null;
let tableSchemas = {}; // Cache for { tableName: { pk: 'id', sk: 'timestamp' } }

async function selectDdbTable(tableName) {
  // Highlight the selected button
  document.querySelectorAll('.table-btn').forEach(b => b.classList.remove('bg-gray-800', 'border-gray-700', 'text-white'));
  event.currentTarget.classList.add('bg-gray-800', 'border-gray-700', 'text-white');

  currentDdbTable = tableName;
  ddbLastEvaluatedKey = null;
  document.getElementById('active-table-title').innerText = tableName;

  if (!tableSchemas[tableName]) {
    try {
      const res = await fetch(`/dashboard/api/dynamo/describe?table=${tableName}`);
      const data = await res.json();

      const schema = { pk: null, sk: null };
      data.Table.KeySchema.forEach(key => {
        if (key.KeyType === 'HASH') schema.pk = key.AttributeName;
        if (key.KeyType === 'RANGE') schema.sk = key.AttributeName;
      });
      tableSchemas[tableName] = schema;
    } catch (err) {
      console.error("Failed to describe table schema", err);
    }
  }

  await fetchDdbTableData(); // This handles the Scan call
}

async function ddbLoadTables() {
  try {
    const res = await fetch('/dashboard/api/dynamo/tables');
    const data = await res.json();
    const container = document.getElementById('ddb-table-list');

    if (data !== undefined && data.TableNames !== undefined) {

      container.innerHTML = data.TableNames.map(name => `
            <button onclick="selectDdbTable('${name}')" class="table-btn w-full text-left p-2 rounded-lg text-sm transition-all hover:bg-gray-800 border border-transparent hover:border-gray-700 text-gray-400 hover:text-white flex justify-between items-center group">
                ${name}
                <span class="opacity-0 group-hover:opacity-100 text-[10px] text-orange-500">View →</span>
            </button>
        `).join('');
    }
  } catch (err) {
    console.error("Failed to load DynamoDB tables", err);
  }
}

async function fetchDdbTableData(next = false) {
  if (!currentDdbTable) return;

  const ddbRefreshIcon = document.getElementById('ddb-refresh-icon');
  const ddbRowCount = document.getElementById('ddb-row-count');
  if (ddbRefreshIcon) {
    ddbRefreshIcon.classList.add('animate-spin'); // Start spinning
  }

  try {
    const payload = {
      TableName: currentDdbTable,
      Limit: 50
    };

    if (next && ddbLastEvaluatedKey) {
      payload.ExclusiveStartKey = ddbLastEvaluatedKey;
    }

    if (ddbActiveFilter) {
      Object.assign(payload, ddbActiveFilter);
    }

    const res = await fetch('/dashboard/api/dynamo/scan', {
      method: 'POST',
      body: JSON.stringify(payload)
    });

    const result = await res.json();
    ddbLastEvaluatedKey = result.LastEvaluatedKey;

    const items = result.Items.map(item => DynamoDbParser.unmarshall({M: item}));
    if (ddbRowCount) {
      ddbRowCount.innerText = `${items.length} Item${items.length === 1 ? '' : 's'}`;
    }
    renderDdbTableGrid(items);
    // Update the column dropdown based on the keys we found
    updateDdbFilterDropdown(items);
    renderDdbPagination(!!ddbLastEvaluatedKey); // Helper to show/hide next button
  } catch (err) {
    console.error("FetchTableData: Refresh failed:", err);
    if (ddbRowCount) {
      ddbRowCount.innerText = "Error";
    }
  } finally {
    // Stop spinning after a small delay for visual feedback
    setTimeout(() => {
      if (ddbRefreshIcon) {
        ddbRefreshIcon.classList.remove('animate-spin');
      }
    }, 500);
  }
}

function renderDdbPagination(hasMore) {
  const container = document.getElementById('ddb-pagination-controls');
  if (hasMore) {
    container.innerHTML = `
            <button onclick="fetchDdbTableData(true)" class="bg-gray-800 hover:bg-orange-600 text-white px-3 py-1 rounded text-xs font-bold transition-colors flex items-center gap-1">
                Load More
                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path d="M9 5l7 7-7 7" stroke-width="3"></path></svg>
            </button>
        `;
  } else {
    container.innerHTML = `<span class="text-[10px] text-gray-600 font-bold uppercase py-1 px-3">End of Table</span>`;
  }
}

function updateDdbFilterDropdown(items) {
  const keys = [...new Set(items.flatMap(obj => Object.keys(obj)))];
  const select = document.getElementById('ddb-filter-column');
  const currentVal = select.value;

  select.innerHTML = '<option value="">Select Column</option>' +
      keys.map(k => `<option value="${k}" ${k === currentVal ? 'selected' : ''}>${k}</option>`).join('');
}

async function toggleDdbAutoRefresh() {
  const isChecked = document.getElementById('ddb-auto-refresh-check').checked;

  if (isChecked) {
    console.log("Auto-refresh enabled (10s)");
    ddbRefreshInterval = setInterval(() => {
      if (currentDdbTable) {
        fetchDdbTableData();
      }
    }, 10000);
  } else {
    console.log("Auto-refresh disabled");
    clearInterval(ddbRefreshInterval);
  }
}

function renderDdbTableGrid(items) {
  // if (items.length === 0) return;

  // 1. Get all unique keys for headers
  const keys = [...new Set(items.flatMap(obj => Object.keys(obj)))];
  const schema = tableSchemas[currentDdbTable] || {};

  // 2. Render Headers
  document.getElementById('ddb-grid-header').innerHTML = `
        <tr>
            ${keys.map(k => `
                <th class="p-3 text-gray-400 border-b border-gray-700 whitespace-nowrap bg-gray-800">
                    <div class="flex items-center gap-1">
                        ${(k === schema.pk || k === schema.sk) ? '<span class="text-orange-500">🔑</span>' : ''}
                        ${k}
                    </div>
                </th>
            `).join('')}
            <th class="p-3 text-gray-400 border-b border-gray-700 text-right bg-gray-800 sticky right-0 z-20 shadow-[-10px_0_15px_-3px_rgba(0,0,0,0.4)]">Actions</th>
        </tr>
    `;

  // 3. Render Rows
  document.getElementById('ddb-grid-body').innerHTML = items.map(item => {
    const itemStr = encodeURIComponent(JSON.stringify(item));
    return `
        <tr class="hover:bg-gray-800/30 group transition-colors">
            ${keys.map(k => {
              const val = item[k];
              const displayVal = (typeof val === 'object' && val !== null) ? JSON.stringify(val) : (val ?? '-');
        
              // Create a safe string for the onclick handler
              const safeVal = encodeURIComponent(JSON.stringify(val));
              return `
                <td class="p-3 text-slate-300 border-b border-gray-800/50 whitespace-nowrap overflow-hidden text-ellipsis max-w-[250px]">
                  <div onclick="openDdbCellModal('${k}', JSON.parse(decodeURIComponent('${safeVal}')))" 
                       class="cursor-pointer hover:text-orange-400 transition-colors"
                       title="Click to expand">
                      ${displayVal}
                  </div>
                </td>`;
            }).join('')}
            <td class="p-3 border-b border-gray-800/50 text-right space-x-2 whitespace-nowrap">
              <button onclick="duplicateDdbItem('${itemStr}')" class="text-emerald-400 hover:text-emerald-300 transition-colors inline-block" title="Duplicate">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2"></path>
                </svg>
              </button>

              <button onclick="openDdbItemEditModal('${itemStr}')" class="text-blue-400 hover:text-blue-300 transition-colors inline-block" title="Edit">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"></path>
                </svg>
              </button>

              <button onclick="deleteDdbItem('${itemStr}')" class="text-red-500 hover:text-red-400 transition-colors inline-block" title="Delete">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
                </svg>
              </button>
            </td>
        </tr>
    `}).join('');

}

function openDdbCellModal(columnName, value) {
  const modal = document.getElementById('ddb-cell-modal');
  const title = document.getElementById('ddb-cell-modal-title');
  const content = document.getElementById('ddb-cell-modal-content');

  title.innerText = `Field: ${columnName}`;

  // Attempt to pretty-print if it looks like JSON/Object
  if (typeof value === 'object' && value !== null) {
    content.innerText = JSON.stringify(value, null, 2);
  } else {
    content.innerText = value;
  }

  modal.classList.remove('hidden');
}

function closeDdbCellModal() {
  document.getElementById('ddb-cell-modal').classList.add('hidden');
}

function copyDdbCellContent() {
  const content = document.getElementById('ddb-cell-modal-content').innerText;
  navigator.clipboard.writeText(content);

  const btn = event.target;
  const originalText = btn.innerText;
  btn.innerText = "Copied!";
  setTimeout(() => btn.innerText = originalText, 2000);
}

function openDdbItemCreateModal() {
  document.getElementById('ddb-item-create-modal').classList.remove('hidden');
  document.getElementById('ddb-item-active-modal-title').innerText = "New Item";
  document.getElementById('ddb-item-json').value = '{\n  "id": "' + Math.random().toString(36).substr(2, 9) + '"\n}';
}

function closeDdbItemCreateModal() {
  document.getElementById('ddb-item-create-modal').classList.add('hidden');
  document.getElementById('ddb-item-modal-error').classList.add('hidden');
}

async function saveDdbItem() {
  const rawJson = document.getElementById('ddb-item-json').value;
  const errorEl = document.getElementById('ddb-item-modal-error');

  try {
    const itemObj = JSON.parse(rawJson);
    const marshalledItem = {};

    // Use our DynamoParser from previous step
    for (let key in itemObj) {
      marshalledItem[key] = DynamoDbParser.marshall(itemObj[key]);
    }

    const res = await fetch('/dashboard/api/dynamo/put_item', {
      method: 'POST',
      body: JSON.stringify({
        TableName: currentDdbTable,
        Item: marshalledItem
      })
    });

    if (!res.ok) throw new Error(await res.text());

    closeDdbItemCreateModal();
    await fetchDdbTableData(); // Refresh grid
  } catch (err) {
    errorEl.innerText = err.message;
    errorEl.classList.remove('hidden');
  }
}

async function deleteDdbItem(itemJsonString) {
  if (!confirm("Delete this item permanently?")) return;

  try {
    const item = JSON.parse(decodeURIComponent(itemJsonString));
    const schema = tableSchemas[currentDdbTable];

    if (!schema || !schema.pk) throw new Error("Table schema not loaded");

    const delPayload = {};
    delPayload[schema.pk] = DynamoDbParser.marshall(item[schema.pk]);

    if (schema.sk && item[schema.sk] !== undefined) {
      delPayload[schema.sk] = DynamoDbParser.marshall(item[schema.sk]);
    }

    const res = await fetch('/dashboard/api/dynamo/delete', {
      method: 'POST',
      body: JSON.stringify({
        TableName: currentDdbTable,
        Key: delPayload
      })
    });

    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.Message || "Delete failed");
    }

    await fetchDdbTableData(); // Refresh the grid
  } catch (err) {
    alert(err.message);
  }
}

function openDdbItemEditModal(itemJsonString) {
  const item = JSON.parse(decodeURIComponent(itemJsonString));
  document.getElementById('ddb-item-create-modal').classList.remove('hidden');
  document.getElementById('ddb-item-active-modal-title').innerText = "Edit Item";

  // Fill the textarea with the current item's standard JSON
  document.getElementById('ddb-item-json').value = JSON.stringify(item, null, 2);
}

function duplicateDdbItem(itemJsonString) {
  const item = JSON.parse(decodeURIComponent(itemJsonString));

  // Open the same modal we use for Create/Edit
  document.getElementById('ddb-item-create-modal').classList.remove('hidden');
  document.getElementById('ddb-item-active-modal-title').innerText = "Duplicate Item";

  // Fill the textarea with the current item's JSON
  // The user will need to change the ID/Partition Key before clicking Save
  document.getElementById('ddb-item-json').value = JSON.stringify(item, null, 2);

  // Focus the textarea so they can start editing immediately
  document.getElementById('ddb-item-json').focus();
}

async function applyDdbFilter() {
  const col = document.getElementById('ddb-filter-column').value;
  const op = document.getElementById('ddb-filter-operator').value;
  const val = document.getElementById('ddb-filter-value').value;

  if (!col || !val) return;

  // Build DynamoDB Filter Expression
  let expression = "";
  const attrValues = { ":val": DynamoDbParser.marshall(val) };
  const attrNames = { "#col": col };

  if (op === "contains") {
    expression = `contains(#col, :val)`;
  } else if (op === "BEGINS_WITH") {
    expression = `begins_with(#col, :val)`;
  } else {
    expression = `#col ${op} :val`;
  }

  ddbActiveFilter = {
    FilterExpression: expression,
    ExpressionAttributeNames: attrNames,
    ExpressionAttributeValues: attrValues
  };

  ddbLastEvaluatedKey = null; // Reset pagination
  await fetchDdbTableData();
}

async function clearDdbFilter() {
  ddbActiveFilter = null;
  document.getElementById('ddb-filter-value').value = "";
  await fetchDdbTableData();
}

// system overview
let eventSource = null;

async function showView(viewId) {
  // Hide all views
  document.querySelectorAll('.view-container').forEach(el => el.classList.add('hidden'));
  // Show selected view
  if (viewId !== 'dynamodb') {
    document.getElementById('ddb-auto-refresh-check').checked = false;
    clearInterval(ddbRefreshInterval);
  }

  document.getElementById(`view-${viewId}`).classList.remove('hidden');
  if (viewId === 'overview') {
    await fetchOverviewData();
  }
  // Update UI if switching to DynamoDB
  if (viewId === 'dynamodb') {
    await ddbLoadTables();
  }

}

async function triggerAction(action, service, target) {
  if (!confirm(`Are you sure you want to ${action.replace('_',' ')} for ${target}?`)) return;

  await fetch('/dashboard/api/action', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({ action, service, target })
  });
  await fetchOverviewData(); // Refresh UI

}

async function fetchOverviewData() {
  try {
    const res = await fetch('/dashboard/api/status');
    const data = await res.json();

    // 1. Update Sidebar Status
    const serviceList = document.getElementById('service-list');
    serviceList.innerHTML = data.services.map(s => `
                    <div class="flex items-center justify-between group">
                        <span class="text-sm font-medium ${s.healthy ? 'text-slate-200' : 'text-gray-600'}">${s.name}</span>
                        <div class="flex items-center gap-2">
                            <div class="w-2 h-2 rounded-full ${s.healthy ? 'bg-green-500 shadow-[0_0_8px_#22c55e]' : 'bg-red-500 status-pulse'}"></div>
                        </div>
                    </div>
                `).join('');

    // 2. Update Storage Grid
    const storageGrid = document.getElementById('storage-grid');
    storageGrid.innerHTML = Object.entries(data.storage).map(([svc, size]) => `
                    <div class="bg-stone-900 border border-stone-800 p-4 rounded-2xl hover:border-stone-700 transition-colors">
                        <div class="text-xs font-bold text-gray-400 uppercase tracking-tighter">${svc}</div>
                        <div class="text-2xl font-black text-white mt-2">${size}</div>
                    </div>
                `).join('');

    document.getElementById('vol-path').innerText = data.volume_path;

    if (data.buckets !== undefined) {
      const s3Container = document.getElementById('s3-actions');
      s3Container.innerHTML = data.buckets.map(b => `
        <div class="flex items-center justify-between p-2 hover:bg-stone-800 rounded-lg transition-colors">
            <span class="text-sm font-mono">${b}</span>
            <button onclick="triggerAction('clear_s3', 's3', '${b}')" class="text-xs bg-red-900/40 text-red-400 px-3 py-1 rounded-md border border-red-500/30 hover:bg-red-500 hover:text-white transition-all">Clear</button>
        </div>
      `).join('');
    }
  } catch (err) {
    console.error("Dashboard sync error:", err);
  }

}

function downloadLogs() {
  const service = document.getElementById('log-selector').value;
  // Simply navigate the window to the download URL
  window.location.href = `/dashboard/api/logs/download?service=${service}`;

}

function switchLog() {
  const service = document.getElementById('log-selector').value;
  const consoleEl = document.getElementById('log-console');

  // Close existing stream
  if (eventSource) eventSource.close();
  consoleEl.innerHTML = `<div class="text-blue-400 font-bold pb-1">[SYSTEM] Switched to ${service} logs...</div>`;

  // Start new SSE connection
  eventSource = new EventSource(`/dashboard/api/logs?service=${service}`);

  eventSource.onmessage = function(event) {
    const line = document.createElement('div');
    // line.className = "border-b border-stone-900 py-1";
    line.className = "pb-1";
    line.textContent = event.data;
    consoleEl.appendChild(line);

    // Auto-scroll to bottom
    consoleEl.scrollTop = consoleEl.scrollHeight;
  };

  eventSource.onerror = function() {
    console.error("Log stream lost. Reconnecting...");
  };
}

// Initialize with Edge logs
switchLog();

// Poll every 3 seconds
// setInterval(fetchData, 3000);
// fetchData();
showView('overview');