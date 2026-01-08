// Dynamodb
const DynamoDbParser = {
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

async function selectDdbTable(tableName) {
  // Highlight the selected button
  document.querySelectorAll('.table-btn').forEach(
      b => b.classList.remove('bg-gray-800', 'border-gray-700', 'text-white'));
  event.currentTarget.classList.add('bg-gray-800', 'border-gray-700',
      'text-white');

  currentDdbTable = tableName;
  ddbLastEvaluatedKey = null;
  document.getElementById('active-table-title').innerText = tableName;

  if (!tableSchemas[tableName]) {
    try {
      const res = await fetch(
          `/dashboard/api/dynamo/describe?table=${tableName}`);
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
      ddbRowCount.innerText = `${items.length} Item${items.length === 1 ? ''
          : 's'}`;
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
      keys.map(k => `<option value="${k}" ${k === currentVal ? 'selected'
          : ''}>${k}</option>`).join('');
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
                        ${(k === schema.pk || k === schema.sk)
      ? '<span class="text-orange-500">🔑</span>' : ''}
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
      const displayVal = (typeof val === 'object' && val !== null)
          ? JSON.stringify(val) : (val ?? '-');

      // Create a safe string for the onclick handler
      const safeVal = encodeURIComponent(JSON.stringify(val));
      return `
                <td class="p-3 text-slate-300 border-b border-gray-800/50 whitespace-nowrap overflow-hidden text-ellipsis max-w-[250px]">
                  <div onclick="openCellModal('${k}', JSON.parse(decodeURIComponent('${safeVal}')))" 
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
    `
  }).join('');

}

function openDdbItemCreateModal() {
  document.getElementById('ddb-item-create-modal').classList.remove('hidden');
  document.getElementById('ddb-item-active-modal-title').innerText = "New Item";
  document.getElementById('ddb-item-json').value = '{\n  "id": "'
      + Math.random().toString(36).substr(2, 9) + '"\n}';
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

async function deleteDdbItem(itemJsonString) {
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

function openDdbItemEditModal(itemJsonString) {
  const item = JSON.parse(decodeURIComponent(itemJsonString));
  document.getElementById('ddb-item-create-modal').classList.remove('hidden');
  document.getElementById(
      'ddb-item-active-modal-title').innerText = "Edit Item";

  // Fill the textarea with the current item's standard JSON
  document.getElementById('ddb-item-json').value = JSON.stringify(item, null,
      2);
}

function duplicateDdbItem(itemJsonString) {
  const item = JSON.parse(decodeURIComponent(itemJsonString));

  // Open the same modal we use for Create/Edit
  document.getElementById('ddb-item-create-modal').classList.remove('hidden');
  document.getElementById(
      'ddb-item-active-modal-title').innerText = "Duplicate Item";

  // Fill the textarea with the current item's JSON
  // The user will need to change the ID/Partition Key before clicking Save
  document.getElementById('ddb-item-json').value = JSON.stringify(item, null,
      2);

  // Focus the textarea so they can start editing immediately
  document.getElementById('ddb-item-json').focus();
}

async function applyDdbFilter() {
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

async function clearDdbFilter() {
  ddbActiveFilter = null;
  document.getElementById('ddb-filter-value').value = "";
  await fetchDdbTableData();
}

// Secrets manager
let smCurrentSecret = null;
let smCurrentSecretValue = "";
let secretModalMode = 'create'

async function loadSecrets() {
  const res = await fetch('/dashboard/api/secrets/list');
  const data = await res.json();
  const container = document.getElementById('sm-secrets-list');

  container.innerHTML = data.SecretList.map(s => `
        <button onclick="selectSecret('${s.Name}')" class="w-full text-left px-4 py-3 rounded-lg text-sm transition-all hover:bg-gray-800 group border border-transparent hover:border-gray-700">
            <div class="text-slate-300 font-medium">${s.Name}</div>
            <div class="text-[10px] text-gray-500 truncate">${s.ARN}</div>
        </button>
    `).join('');
}

async function selectSecret(name) {
  const res = await fetch(`/dashboard/api/secrets/get?name=${name}`);
  const data = await res.json();

  smCurrentSecret = name;
  smCurrentSecretValue = data.SecretString;

  document.getElementById('sm-secret-details').classList.remove('hidden');
  document.getElementById('sm-active-secret-name').innerText = name;
  document.getElementById('sm-active-secret-arn').innerText = data.ARN;

  const display = document.getElementById('sm-secret-value-display');
  display.innerText = smCurrentSecretValue;
  display.classList.add('blur-sm'); // Always start blurred
  document.getElementById('sm-visibility-btn').innerText = "Reveal";
}

function toggleSecretVisibility() {
  const display = document.getElementById('sm-secret-value-display');
  const btn = document.getElementById('sm-visibility-btn');

  if (display.classList.contains('blur-sm')) {
    display.classList.remove('blur-sm', 'select-none');
    btn.innerText = "Hide";
  } else {
    display.classList.add('blur-sm', 'select-none');
    btn.innerText = "Reveal";
  }
}

function openSecretModal(mode) {
  secretModalMode = mode;
  const modal = document.getElementById('secret-modal');
  const nameInput = document.getElementById('sm-modal-secret-name');
  const valueInput = document.getElementById('sm-modal-secret-value');
  const title = document.getElementById('sm-secret-modal-title');
  const nameGroup = document.getElementById('sm-secret-name-group');

  modal.classList.remove('hidden');
  document.getElementById('sm-secret-modal-error').classList.add('hidden');

  if (mode === 'edit') {
    title.innerText = "Edit Secret Value";
    nameGroup.classList.add('hidden'); // Cannot rename via PutSecretValue
    valueInput.value = smCurrentSecretValue;
  } else {
    title.innerText = "Create New Secret";
    nameGroup.classList.remove('hidden');
    nameInput.value = "";
    valueInput.value = "";
  }
}

function closeSecretModal() {
  document.getElementById('secret-modal').classList.add('hidden');
}

function formatSecretJson() {
  const input = document.getElementById('sm-modal-secret-value');
  try {
    const obj = JSON.parse(input.value);
    input.value = JSON.stringify(obj, null, 2);
  } catch (e) {
    alert("Not a valid JSON string");
  }
}

async function saveSecret() {
  const name = document.getElementById('sm-modal-secret-name').value;
  const value = document.getElementById('sm-modal-secret-value').value;
  const errorEl = document.getElementById('sm-secret-modal-error');

  // For 'create', we use a specific API. For 'edit', we use another.
  const endpoint = secretModalMode === 'create'
      ? '/dashboard/api/secrets/create' : '/dashboard/api/secrets/put';

  const payload = secretModalMode === 'create'
      ? {Name: name, SecretString: value}
      : {SecretId: smCurrentSecret, SecretString: value};

  try {
    const res = await fetch(endpoint, {
      method: 'POST',
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      throw new Error(await res.text());
    }

    closeSecretModal();
    await loadSecrets(); // Refresh sidebar
    if (secretModalMode === 'edit') {
      await selectSecret(smCurrentSecret); // Refresh view
    }
  } catch (err) {
    errorEl.innerText = err.message;
    errorEl.classList.remove('hidden');
  }
}

async function deleteSecret() {
  if (!confirm(`Are you sure you want to delete ${smCurrentSecret}?`)) {
    return;
  }

  try {
    const res = await fetch(
        `/dashboard/api/secrets/delete?name=${smCurrentSecret}`, {
          method: 'POST'
        });

    if (!res.ok) {
      throw new Error("Failed to delete secret");
    }

    document.getElementById('sm-secret-details').classList.add('hidden');
    smCurrentSecret = null;
    await loadSecrets();
  } catch (err) {
    alert(err.message);
  }
}

// KMS manager
let currentKmsKey = null;

async function loadKmsKeys() {
  const res = await fetch('/dashboard/api/kms/list');
  const data = await res.json();
  const container = document.getElementById('kms-key-list');

  if (data && data.Keys) {
    container.innerHTML = data.Keys.map(k => `
        <button onclick="selectKmsKey('${k.KeyId}')" class="w-full text-left px-4 py-3 rounded-lg text-sm transition-all hover:bg-gray-800 group border border-transparent hover:border-gray-700">
            <div class="text-slate-300 font-mono text-[11px] truncate">${k.KeyId}</div>
        </button>
    `).join('');
  }
}

function selectKmsKey(id) {
  currentKmsKey = id;
  document.getElementById('kms-workspace').classList.remove('hidden');
  document.getElementById('kms-active-key-id').innerText = id;
}

async function kmsEncrypt() {
  const text = document.getElementById('kms-encrypt-input').value;
  const res = await fetch('/dashboard/api/kms/encrypt', {
    method: 'POST',
    body: JSON.stringify({
      KeyId: currentKmsKey,
      Plaintext: btoa(text) // AWS KMS expects base64 plaintext
    })
  });
  const data = await res.json();
  document.getElementById('kms-encrypt-output').innerText = data.CiphertextBlob;
}

async function kmsDecrypt() {
  const blob = document.getElementById('kms-decrypt-input').value;
  const res = await fetch('/dashboard/api/kms/decrypt', {
    method: 'POST',
    body: JSON.stringify({CiphertextBlob: blob})
  });
  const data = await res.json();
  // Decode base64 result back to text
  document.getElementById('kms-decrypt-output').innerText = atob(
      data.Plaintext);
}

async function createKmsKey() {
  await fetch('/dashboard/api/kms/create', {method: 'POST'});
  await loadKmsKeys();
}

function copyKmsResult(elementId) {
  const text = document.getElementById(elementId).innerText;
  if (!text) {
    return;
  }

  navigator.clipboard.writeText(text).then(() => {
    const btn = event.currentTarget;
    const originalHtml = btn.innerHTML;

    // Change to a "Checkmark" icon temporarily
    btn.innerHTML = `<svg class="w-3 h-3 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7"/></svg>`;

    setTimeout(() => {
      btn.innerHTML = originalHtml;
    }, 2000);
  });
}

// SQS Manager
let sqsCurrentQueueUrl = "";

async function sqsLoadQueues() {
  const res = await fetch('/dashboard/api/sqs/list');
  const data = await res.json();
  const urls = data.QueueUrls || [];
  const container = document.getElementById('sqs-queue-list');

  container.innerHTML = '';
  for (const url of urls) {
    const name = url.split('/').pop();
    // Fetch counts for each queue
    const attrRes = await fetch(
        `/dashboard/api/sqs/attributes?url=${encodeURIComponent(url)}`);
    const attrData = await attrRes.json();
    const count = attrData.Attributes.ApproximateNumberOfMessages;

    container.innerHTML += `
            <button onclick="sqsSelectQueue('${url}', '${name}')" class="w-full text-left p-3 rounded-lg border border-gray-800/50 hover:bg-gray-800 transition-all group">
                <div class="text-slate-300 text-xs font-bold truncate">${name}</div>
                <div class="flex justify-between items-center mt-1">
                    <span class="text-[10px] text-gray-500">Messages</span>
                    <span class="text-[10px] px-1.5 py-0.5 bg-orange-900/30 text-orange-400 rounded font-mono font-bold">${count}</span>
                </div>
            </button>`;
  }
}

function sqsSelectQueue(url, name) {
  sqsCurrentQueueUrl = url;

  // Update UI headers
  document.getElementById('sqs-active-queue-name').innerText = name;
  document.getElementById('sqs-active-queue-url').innerText = url;

  // Show the workspace and clear the previous message list
  document.getElementById('sqs-workspace').classList.remove('hidden');
  document.getElementById('sqs-messages').innerHTML = `
        <div class="text-center text-gray-600 text-xs mt-10 italic">
            Click Poll to peek at messages in ${name}
        </div>
    `;

  // Optional: Auto-poll when selecting a queue
  // receiveMessages();
}

async function sqsPurgeQueue() {
  const queueName = document.getElementById('sqs-active-queue-name').innerText;

  if (!confirm(
      `Are you sure you want to PURGE all messages in "${queueName}"? This cannot be undone.`)) {
    return;
  }

  try {
    const res = await fetch(
        `/dashboard/api/sqs/purge?url=${encodeURIComponent(currentQueueUrl)}`, {
          method: 'POST'
        });

    if (!res.ok) {
      throw new Error("Failed to purge queue");
    }

    // UI Feedback
    document.getElementById('sqs-messages').innerHTML = `
            <div class="text-center text-emerald-500 text-xs mt-10 font-bold">
                Queue purged successfully.
            </div>
        `;

    // Refresh the sidebar counts after a short delay (SQS counts are eventual)
    setTimeout(sqsLoadQueues, 1000);

  } catch (err) {
    alert("Error purging queue: " + err.message);
  }
}

async function sqsReceiveMessages() {
  const res = await fetch(`/dashboard/api/sqs/receive?url=${encodeURIComponent(sqsCurrentQueueUrl)}`);
  const data = await res.json();
  const container = document.getElementById('sqs-messages');

  if (!data.Messages || data.Messages.length === 0) {
    container.innerHTML = '<div class="text-center text-gray-700 text-xs mt-10">No messages found</div>';
    return;
  }

  container.innerHTML = data.Messages.map(m => {
    // Encode the ReceiptHandle as it can contain special characters
    const handle = encodeURIComponent(m.ReceiptHandle);
    return `
        <div class="bg-gray-900 border border-gray-800 rounded-lg p-3 relative group">
            <div class="flex justify-between items-center mb-2">
                <span class="text-[9px] text-gray-500 font-mono">ID: ${m.MessageId.substring(0, 8)}...</span>
                <button onclick="sqsDeleteMessage('${handle}')" 
                        class="text-red-500 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity" 
                        title="Delete Message">
                    <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                </button>
            </div>
            <pre class="text-[11px] text-orange-200 overflow-x-auto whitespace-pre-wrap">${m.Body}</pre>
        </div>
    `
  }).join('');
}

async function sqsSendMessage() {
  const body = document.getElementById('sqs-send-body').value;
  await fetch('/dashboard/api/sqs/send', {
    method: 'POST',
    body: JSON.stringify({QueueUrl: sqsCurrentQueueUrl, MessageBody: body})
  });
  document.getElementById('sqs-send-body').value = '';
  await sqsLoadQueues(); // Refresh counts
}

async function sqsDeleteMessage(encodedHandle) {
  if (!confirm("Delete this specific message?")) {
    return;
  }

  try {
    const res = await fetch(
        `/dashboard/api/sqs/delete-message?url=${encodeURIComponent(sqsCurrentQueueUrl)}&handle=${encodedHandle}`, {
          method: 'POST'
        });

    if (!res.ok) {
      throw new Error("Failed to delete message");
    }

    // Refresh the list to show it's gone
    await sqsReceiveMessages();
    // Update sidebar counts (delayed slightly to allow SQS to sync)
    setTimeout(sqsLoadQueues, 500);

  } catch (err) {
    alert("Error: " + err.message);
  }
}

// S3 Manager
let s3CurrentBucket = "";
let s3CurrentBucketPath = "";

async function s3LoadBuckets() {
  const res = await fetch('/dashboard/api/s3/list-buckets');
  const data = await res.json();
  const container = document.getElementById('s3-bucket-list');

  if (data && data.Buckets) {
    container.innerHTML = data.Buckets.map(b => `
        <div class="flex items-center group px-2 rounded-lg hover:bg-gray-800 transition-all">
            <button onclick="s3SelectBucket('${b.Name}', '${b.Path}')" class="flex-1 text-left py-3 text-sm flex items-center gap-2 overflow-hidden">
                <svg class="w-5 h-5 text-yellow-500 shrink-0" fill="currentColor" viewBox="0 0 20 20"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>
                <span class="text-slate-300 truncate">${b.Name}</span>
            </button>
            
            <button onclick="s3DeleteBucket('${b.Name}')" class="p-2 text-gray-400 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity">
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
            </button>
        </div>
    `).join('');
  }
}

// Validation Logic
function s3ValidateBucketName(name) {
  const regex = /^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$/;
  const isValid = regex.test(name) && !name.includes('..');
  const btn = document.getElementById('s3-btn-confirm-create');
  const hint = document.getElementById('s3-bucket-hint');

  btn.disabled = !isValid;
  hint.className = isValid ? "mt-2 text-[9px] text-emerald-500" : "mt-2 text-[9px] text-red-500";
}

function s3OpenCreateBucketModal() {
  console.log('s3OpenCreateBucketModal');
  document.getElementById('s3-create-bucket-modal').classList.remove('hidden');
  document.getElementById('s3-new-bucket-name').value = "";
  s3ValidateBucketName("");
}

function s3CloseCreateBucketModal() {
  document.getElementById('s3-create-bucket-modal').classList.add('hidden');
}

async function s3PerformCreateBucket() {
  const name = document.getElementById('s3-new-bucket-name').value;
  const res = await fetch(`/dashboard/api/s3/create-bucket?name=${name}`, {method: 'POST'});
  if (res.ok) {
    s3CloseCreateBucketModal();
    await s3LoadBuckets();
  }
}

async function s3EmptyBucket() {
  if (!confirm(
      `Are you sure you want to DELETE ALL objects in "${s3CurrentBucket}"?`)) {
    return;
  }

  // 1. Fetch all objects
  const res = await fetch(
      `/dashboard/api/s3/list-objects?bucket=${s3CurrentBucket}`);
  const data = await res.json();

  if (!data.Contents || data.Contents.length === 0) {
    alert("Bucket is already empty.");
    return;
  }

  // 2. Delete each object
  // In a production app, you'd use DeleteObjects (plural) API,
  // but for local dev, iterating is fine and shows progress.
  for (const obj of data.Contents) {
    await fetch(
        `/dashboard/api/s3/delete?bucket=${s3CurrentBucket}&key=${encodeURIComponent(
            obj.Key)}`, {
          method: 'POST'
        });
  }

  alert(`Emptied ${data.Contents.length} objects.`);
  await s3SelectBucket(s3CurrentBucket, s3CurrentBucketPath); // Refresh view
}

async function s3DeleteBucket(name) {
  if (!confirm(
      `Delete bucket "${name}"? Note: Bucket must be empty first.`)) {
    return;
  }

  const res = await fetch(
      `/dashboard/api/s3/delete-bucket?name=${encodeURIComponent(name)}`,
      {method: 'POST'});
  if (res.ok) {
    if (s3CurrentBucket === name) {
      document.getElementById('s3-workspace').classList.add('hidden');
      s3CurrentBucket = "";
    }
    await s3LoadBuckets();
  } else {
    const err = await res.json();
    alert(`Error: ${err.Message || "Bucket might not be empty"}`);
  }
}

async function s3SelectBucket(name, path) {
  s3CurrentBucket = name;
  s3CurrentBucketPath = path;
  document.getElementById('s3-active-bucket-name').innerText = name;
  document.getElementById('s3-active-bucket-path').innerText = path;
  document.getElementById('s3-workspace').classList.remove('hidden');

  const res = await fetch(`/dashboard/api/s3/list-objects?bucket=${name}`);
  const data = await res.json();
  const container = document.getElementById('s3-object-list');

  if (!data.Contents) {
    container.innerHTML = '<tr><td colspan="3" class="p-10 text-center text-gray-600">No objects found</td></tr>';
    return;
  }

  container.innerHTML = data.Contents.map(obj => {
    const safeKey = encodeURIComponent(obj.Key);
    return `
        <tr class="hover:bg-gray-800/30 group border-b border-gray-800/50 transition-colors">
        <td class="p-3">
            <button onclick="s3ViewObject('${safeKey}')" 
                    class="text-blue-400 hover:text-blue-300 hover:underline text-left transition-colors">
                ${obj.Key}
            </button>
        </td>
        <td class="p-3 text-gray-400 font-mono">${(obj.Size / 1024).toFixed(2)} KB</td>
        <td class="p-3 text-right">
            <button onclick="s3DeleteObject('${safeKey}')" class="text-red-500 hover:text-red-400 p-1">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                </svg>
            </button>
        </td>
    </tr>
    `;
  }).join('');
}

function s3OpenUploadModal() {
  document.getElementById('s3-upload-bucket-name').innerText = s3CurrentBucket;
  document.getElementById('s3-upload-modal').classList.remove('hidden');
  document.getElementById('s3-upload-key').value = "";
  document.getElementById('s3-upload-content').value = "";
}

function s3CloseUploadModal() {
  document.getElementById('s3-upload-modal').classList.add('hidden');
}

async function s3PerformUpload() {
  const key = document.getElementById('s3-upload-key').value;
  const content = document.getElementById('s3-upload-content').value;

  if (!key) {
    return alert("Object key is required");
  }

  try {
    const res = await fetch('/dashboard/api/s3/upload', {
      method: 'POST',
      body: JSON.stringify({
        Bucket: s3CurrentBucket,
        Key: key,
        Body: btoa(content) // Encode to base64 for safe transit
      })
    });

    if (!res.ok) {
      throw new Error("Upload failed");
    }

    s3CloseUploadModal();

    await s3SelectBucket(s3CurrentBucket, s3CurrentBucketPath); // Refresh the list

  } catch (err) {
    alert(err.message);
  }
}

async function s3DeleteObject(key) {
  if (!confirm(`Delete ${key} from ${s3CurrentBucket}?`)) {
    return;
  }

  try {
    const res = await fetch(
        `/dashboard/api/s3/delete?bucket=${s3CurrentBucket}&key=${encodeURIComponent(
            key)}`, {
          method: 'POST'
        });

    if (!res.ok) {
      throw new Error("Delete failed");
    }

    await s3SelectBucket(s3CurrentBucket, s3CurrentBucketPath); // Refresh the list

  } catch (err) {
    alert(err.message);
  }
}

async function s3ViewObject(key) {
  // In a real AWS environment, you'd use a Presigned URL.
  // Locally, we can just fetch the object data.
  const res = await fetch(`/dashboard/api/s3/get-object?bucket=${s3CurrentBucket}&key=${encodeURIComponent(key)}`);
  const data = await res.json();

  // Decode from base64 (AWS returns Body as base64 in many JSON proxies)
  const content = atob(data.Body);

  openCellModal(`S3: ${key}`, content);
}

// Cloudwatch
let cwCurrentLogGroup = "";
let cwCurrentLogStream = "";

async function cwLoadLogGroups() {
  const res = await fetch('/dashboard/api/logs/groups');
  const data = await res.json();
  const container = document.getElementById('cw-group-list');

  if (data && data.logGroups) {
    container.innerHTML = data.logGroups.map(g => `
        <button onclick="cwSelectLogGroup('${g.logGroupName}')" class="w-full text-left px-3 py-2 rounded-md text-xs transition-all hover:bg-gray-800 text-slate-400 hover:text-white truncate">
            ${g.logGroupName}
        </button>
    `).join('');
  }
}

async function cwSelectLogGroup(groupName) {
  cwCurrentLogGroup = groupName;
  document.getElementById('cw-streams-panel').classList.remove('hidden');
  document.getElementById('cw-events-panel').classList.add('hidden');

  const res = await fetch(`/dashboard/api/logs/streams?group=${encodeURIComponent(groupName)}`);
  const data = await res.json();
  const container = document.getElementById('cw-stream-list');

  container.innerHTML = data.logStreams.map(s => `
        <button onclick="cwSelectLogStream('${s.logStreamName}')" class="w-full text-left px-3 py-2 rounded-md text-[11px] transition-all hover:bg-gray-800 text-slate-300 hover:text-orange-400 truncate font-mono">
            ${s.logStreamName}
        </button>
    `).join('');
}

async function cwSelectLogStream(streamName) {
  cwCurrentLogStream = streamName;
  document.getElementById('cw-events-panel').classList.remove('hidden');
  document.getElementById('active-stream-name').innerText = streamName;
  await cwRefreshLogs();
}

function cwApplyLogFilter(keyword) {
  const term = keyword.toLowerCase();
  const rows = document.querySelectorAll('#cw-event-list > div');

  rows.forEach(row => {
    // We only search the message part, not the timestamp
    const messageText = row.querySelector('.log-message-body').innerText.toLowerCase();

    if (messageText.includes(term)) {
      row.classList.remove('hidden');
    } else {
      row.classList.add('hidden');
    }
  });
}

async function cwRefreshLogs() {
  const container = document.getElementById('cw-event-list');
  const res = await fetch(`/dashboard/api/logs/events?group=${encodeURIComponent(cwCurrentLogGroup)}&stream=${encodeURIComponent(cwCurrentLogStream)}`);
  const data = await res.json();

  container.innerHTML = data.events.map(e => {
    const date = new Date(e.timestamp).toISOString();
    let message = e.message;

    message = message.replace(/DEBUG/g, '<span class="text-slate-400 font-bold">DEBUG</span>');
    message = message.replace(/ERROR/g, '<span class="text-red-500 font-bold">ERROR</span>');
    message = message.replace(/INFO/g, '<span class="text-blue-400 font-bold">INFO</span>');
    message = message.replace(/WARN/g, '<span class="text-yellow-500 font-bold">WARN</span>');

    return `
        <div class="py-1 border-b border-gray-900/30 flex gap-4 group hover:bg-white/5 transition-colors">
            <span class="text-gray-300 shrink-0 select-none text-[10px]">${date}</span>
            <span class="log-message-body text-gray-200 break-all">${message}</span>
        </div>`;
  }).join('');

  // Re-apply any existing filter after refresh
  const currentFilter = document.getElementById('cw-log-filter').value;
  if (currentFilter) {
    cwApplyLogFilter(currentFilter);
  }
  container.scrollTop = container.scrollHeight;
}

// system overview
let eventSource = null;

async function showView(viewId) {
  // Hide all views
  document.querySelectorAll('.view-container').forEach(
      el => el.classList.add('hidden'));
  // Show selected view
  if (viewId !== 'dynamodb') {
    document.getElementById('ddb-auto-refresh-check').checked = false;
    clearInterval(ddbRefreshInterval);
  }

  document.getElementById(`view-${viewId}`).classList.remove('hidden');

  switch (viewId) {
    case "overview":
      await fetchOverviewData();
      return
    case 'dynamodb':
      await ddbLoadTables();
      return;
    case 'secrets':
      await loadSecrets();
      return;
    case 'kms':
      await loadKmsKeys();
      return;
    case 'sqs':
      await sqsLoadQueues();
      return;
    case 's3':
      await s3LoadBuckets();
      return;
    case 'cloudwatch':
      await cwLoadLogGroups()
      return;
  }
}

/*
async function triggerAction(action, service, target) {
  if (!confirm(`Are you sure you want to ${action.replace('_',' ')} for ${target}?`)) return;

  await fetch('/dashboard/api/action', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({ action, service, target })
  });
  await fetchOverviewData(); // Refresh UI

}
*/

async function fetchOverviewData() {
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
                            <div class="text-xs font-bold text-gray-400 uppercase tracking-tighter">${svc}</div>
                            <div class="text-2xl font-black text-white mt-2">${size}</div>
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

function openCellModal(columnName, value) {
  const modal = document.getElementById('cell-modal');
  const title = document.getElementById('cell-modal-title');
  const content = document.getElementById('cell-modal-content');

  title.innerText = `Field: ${columnName}`;

  // Attempt to pretty-print if it looks like JSON/Object
  if (typeof value === 'object' && value !== null) {
    content.innerText = JSON.stringify(value, null, 2);
  } else {
    content.innerText = value;
  }

  modal.classList.remove('hidden');
}

function closeCellModal() {
  document.getElementById('cell-modal').classList.add('hidden');
}

function copyCellContent() {
  const content = document.getElementById('cell-modal-content').innerText;
  navigator.clipboard.writeText(content);

  const btn = event.target;
  const originalText = btn.innerText;
  btn.innerText = "Copied!";
  setTimeout(() => btn.innerText = originalText, 2000);
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

// Initialize with Edge logs
switchLog();

// Poll every 3 seconds
// setInterval(fetchData, 3000);
// fetchData();
showView('overview');