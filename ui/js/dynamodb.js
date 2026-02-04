import {
  styleSelectedElement,
  openCellModal
} from "./utils.js"

// Dynamodb
export const DynamoDbParser = {
  // Convert DynamoDB JSON -> Standard JSON
  unmarshall: function (data) {
    if (!data || typeof data !== 'object') {
      return data;
    }
    if (data.S !== undefined) {
      return data.S;
    }
    if (data.N !== undefined) {
      return Number(data.N);
    }
    if (data.BOOL !== undefined) {
      return data.BOOL;
    }
    if (data.NULL !== undefined) {
      return null;
    }
    if (data.M) {
      let obj = {};
      for (let k in data.M) {
        obj[k] = this.unmarshall(data.M[k]);
      }
      return obj;
    }
    if (data.L) {
      return data.L.map(v => this.unmarshall(v));
    }
    return data;
  },

  // Convert Standard JSON -> DynamoDB JSON (for Create/Update)
  marshall: function (data) {
    if (typeof data === 'string') {
      return {S: data};
    }
    if (typeof data === 'number') {
      return {N: data.toString()};
    }
    if (typeof data === 'boolean') {
      return {BOOL: data};
    }
    if (data === null) {
      return {NULL: true};
    }
    if (Array.isArray(data)) {
      return {L: data.map(v => this.marshall(v))};
    }
    if (typeof data === 'object') {
      let m = {};
      for (let k in data) {
        m[k] = this.marshall(data[k]);
      }
      return {M: m};
    }
    return {S: String(data)};
  }
};

let currentDdbTable = null;
let ddbLastEvaluatedKey = null;
let ddbActiveFilter = null;
let ddbRefreshInterval = null;
let tableSchemas = {}; // Cache for { tableName: { pk: 'id', sk: 'timestamp' } }

export async function selectDdbTable(e, tableName) {
  // Highlight the selected button

  styleSelectedElement(e, 'button.cls-table-btn');

  currentDdbTable = tableName;
  ddbLastEvaluatedKey = null;
  document.getElementById('active-table-title').innerText = tableName;

  if (!tableSchemas[tableName]) {
    try {
      const res = await fetch(`/dashboard/api/dynamo/describe?table=${tableName}`);
      const data = await res.json();

      const schema = {pk: null, sk: null};
      data.Table.KeySchema.forEach(key => {
        if (key.KeyType === 'HASH') {
          schema.pk = key.AttributeName;
        }
        if (key.KeyType === 'RANGE') {
          schema.sk = key.AttributeName;
        }
      });
      tableSchemas[tableName] = schema;
    } catch (err) {
      console.error("Failed to describe table schema", err);
    }
  }

  await fetchDdbTableData(); // This handles the Scan call
}

export async function ddbLoadTables() {
  try {

    const res = await fetch('/dashboard/api/dynamo/tables');
    const data = await res.json();
    const container = document.getElementById('ddb-table-list');

    if (data !== undefined && data.TableNames !== undefined) {

      container.innerHTML = data.TableNames.map(name => `
            <button onclick="selectDdbTable(this, '${name}')" class="cls-table-btn w-full text-left p-2 rounded-lg text-sm transition-all hover:bg-neutral-800 border border-transparent hover:border-neutral-700 text-gray-400 hover:text-white flex justify-between items-center group">
              ${name}
              <span class="opacity-0 group-hover:opacity-100 text-[10px] text-orange-500">View →</span>
            </button>
        `).join('');
    }
  } catch (err) {
    console.error("Failed to load DynamoDB tables", err);
  }
}

export async function fetchDdbTableData(next = false) {
  if (!currentDdbTable) {
    return;
  }

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

    const items = result.Items.map(
        item => DynamoDbParser.unmarshall({M: item}));
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

export function renderDdbPagination(hasMore) {
  const container = document.getElementById('ddb-pagination-controls');
  if (hasMore) {
    container.innerHTML = `
            <button onclick="fetchDdbTableData(true)" class="bg-neutral-800 hover:bg-orange-600 text-white px-3 py-1 rounded text-xs font-bold transition-colors flex items-center gap-1">
                Load More
                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path d="M9 5l7 7-7 7" stroke-width="3"></path></svg>
            </button>
        `;
  } else {
    container.innerHTML = `<span class="text-[10px] text-gray-600 font-bold py-1 px-3">End of Table</span>`;
  }
}

export function updateDdbFilterDropdown(items) {
  const keys = [...new Set(items.flatMap(obj => Object.keys(obj)))];
  const select = document.getElementById('ddb-filter-column');
  const currentVal = select.value;

  select.innerHTML = '<option value="">Select Column</option>' +
      keys.map(k => `<option value="${k}" ${k === currentVal ? 'selected' : ''}>${k}</option>`)
          .join('');
}

export async function toggleDdbAutoRefresh() {
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

export function ddbSchemeExtractKeys(items, schema) {
  const arr = [schema.pk]
  if (schema.sk) {
    arr.push(schema.sk);
  }
  arr.push(...new Set(items.flatMap(obj => Object.keys(obj))))

  return [...new Set(arr)];
}

export function renderDdbTableGrid(items) {

  // 1. Get all unique keys for headers
  const schema = tableSchemas[currentDdbTable] || {};
  const keys = ddbSchemeExtractKeys(items, schema)

  // 2. Render Headers
  document.getElementById('ddb-grid-header').innerHTML = `
        <tr>
            ${keys.map(k => `
                <th class="p-3 text-gray-400 border-b border-neutral-700 whitespace-nowrap bg-neutral-800">
                    <div class="flex items-center gap-1">
                        ${(k === schema.pk || k === schema.sk) 
                            ? '<span class="text-orange-500">🔑</span>' 
                            : ''}
                        ${k}
                    </div>
                </th>
            `).join('')}
            <th class="p-3 text-gray-400 border-b border-neutral-700 text-right bg-neutral-800 sticky right-0 z-20 shadow-[-10px_0_15px_-3px_rgba(0,0,0,0.4)]">Actions</th>
        </tr>
    `;

  // 3. Render Rows
  document.getElementById('ddb-grid-body').innerHTML = items.map(item => {
    const itemStr = encodeURIComponent(JSON.stringify(item));
    return `
        <tr class="hover:bg-neutral-800/30 group transition-colors">
            ${keys.map(k => {
              const val = item[k];
              const displayVal = (typeof val === 'object' && val !== null) 
                  ? JSON.stringify(val) 
                  : (val ?? '-');
              
              // Create a safe string for the onclick handler
              const safeVal = encodeURIComponent(JSON.stringify(val));
              return `
              <td class="p-3 text-slate-300 border-b border-neutral-800/50 whitespace-nowrap overflow-hidden text-ellipsis max-w-[250px]">
                <div onclick="openCellModal('${k}', JSON.parse(decodeURIComponent('${safeVal}')))" class="cursor-pointer hover:text-orange-400 transition-colors" title="Click to expand">
                  ${displayVal}
                </div>
              </td>
            `;
            }).join('')}
            
            <td class="p-3 border-b border-neutral-800/50 bg-neutral-950 text-right space-x-2 whitespace-nowrap sticky right-0 z-20 shadow-[-10px_0_15px_-3px_rgba(0,0,0,0.4)]">
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
    `
  }).join('');

}

export function openDdbItemCreateModal() {
  document.getElementById('ddb-item-create-modal').classList.remove('hidden');
  document.getElementById('ddb-item-active-modal-title').innerText = "New Item";
  document.getElementById('ddb-item-json').value = '{\n  "id": "' + Math.random().toString(36).substr(2, 9) + '"\n}';
}

export function closeDdbItemCreateModal() {
  document.getElementById('ddb-item-create-modal').classList.add('hidden');
  document.getElementById('ddb-item-modal-error').classList.add('hidden');
}

export async function saveDdbItem() {
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

    if (!res.ok) {
      throw new Error(await res.text());
    }

    closeDdbItemCreateModal();
    await fetchDdbTableData(); // Refresh grid
  } catch (err) {
    errorEl.innerText = err.message;
    errorEl.classList.remove('hidden');
  }
}

export async function deleteDdbItem(itemJsonString) {
  if (!confirm("Delete this item permanently?")) {
    return;
  }

  try {
    const item = JSON.parse(decodeURIComponent(itemJsonString));
    const schema = tableSchemas[currentDdbTable];

    if (!schema || !schema.pk) {
      throw new Error("Table schema not loaded");
    }

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

export function openDdbItemEditModal(itemJsonString) {
  const item = JSON.parse(decodeURIComponent(itemJsonString));
  document.getElementById('ddb-item-create-modal').classList.remove('hidden');
  document.getElementById('ddb-item-active-modal-title').innerText = "Edit Item";

  // Fill the textarea with the current item's standard JSON
  document.getElementById('ddb-item-json').value = JSON.stringify(item, null, 2);
}

export function duplicateDdbItem(itemJsonString) {
  const item = JSON.parse(decodeURIComponent(itemJsonString));

  // Open the same modal we use for Create/Edit
  document.getElementById('ddb-item-create-modal').classList.remove('hidden');
  document.getElementById('ddb-item-active-modal-title').innerText = "Clone Item";

  // Fill the textarea with the current item's JSON
  // The user will need to change the ID/Partition Key before clicking Save
  document.getElementById('ddb-item-json').value = JSON.stringify(item, null, 2);

  // Focus the textarea so they can start editing immediately
  document.getElementById('ddb-item-json').focus();
}

export async function applyDdbFilter() {
  const col = document.getElementById('ddb-filter-column').value;
  const op = document.getElementById('ddb-filter-operator').value;
  const val = document.getElementById('ddb-filter-value').value;

  if (!col || !val) {
    return;
  }

  // Build DynamoDB Filter Expression
  let expression = "";
  const attrValues = {":val": DynamoDbParser.marshall(val)};
  const attrNames = {"#col": col};

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

export async function clearDdbFilter() {
  ddbActiveFilter = null;
  document.getElementById('ddb-filter-value').value = "";
  await fetchDdbTableData();
}