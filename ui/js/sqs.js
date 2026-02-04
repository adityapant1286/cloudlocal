import {
  styleSelectedElement,
  openCellModal,
  closeCellModal
} from "./utils.js"

// SQS Manager
let sqsCurrentQueueUrl = "";

export async function sqsLoadQueues() {
  const res = await fetch('/dashboard/api/sqs/list');
  const data = await res.json();
  const queues = data.Queues || [];
  const container = document.getElementById('sqs-queue-list');

  container.innerHTML = '';
  for (const qu of queues) {
    const name = qu.URL.split('/').pop();
    const count = qu.Messages ? qu.Messages.length : 0;

    container.innerHTML += `
            <button onclick="selectSqsQueue(this, '${qu.URL}', '${name}')" class="cls-sqs-queue w-full text-left p-3 rounded-lg border border-neutral-800/50 hover:bg-neutral-800 transition-all group">
                <div class="text-slate-300 text-sm font-bold truncate">${name}</div>
                <div class="flex justify-between items-center mt-1">
                    <span class="text-[10px] text-gray-500">Messages</span>
                    <span class="text-sm px-1.5 py-0.5 bg-orange-900/30 text-orange-400 rounded font-mono font-bold">${count}</span>
                </div>
            </button>
    `;
  }
}

export function sqsOpenCreateQueueModal() {
  document.getElementById('sqs-create-queue-modal').classList.remove('hidden');
  document.getElementById('sqs-new-queue-name').value = "";
}

export function sqsCloseCreateQueueModal() {
  document.getElementById('sqs-create-queue-modal').classList.add('hidden');
}

export async function sqsPerformCreateQueue() {
  const name = document.getElementById('sqs-new-queue-name').value;
  const res = await fetch(
      `/dashboard/api/sqs/create-queue?name=${name}`,
      {method: 'POST'}
  );
  if (res.ok) {
    sqsCloseCreateQueueModal();
    await sqsLoadQueues();
  }
}

export function selectSqsQueue(e, url, name) {
  sqsCurrentQueueUrl = url;

  // Update UI headers
  document.getElementById('sqs-active-queue-name').innerText = name;
  document.getElementById('sqs-active-queue-url').innerText = url;

  styleSelectedElement(e, 'button.cls-sqs-queue');

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

export async function sqsPurgeQueue() {
  const queueName = document.getElementById('sqs-active-queue-name').innerText;

  if (!confirm(`Are you sure you want to PURGE all messages in "${queueName}"? This cannot be undone.`)) {
    return;
  }

  try {
    const res = await fetch(
        `/dashboard/api/sqs/purge?url=${encodeURIComponent(sqsCurrentQueueUrl)}`,
        {method: 'POST'}
    );

    if (!res.ok) {
      console.error(res);
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

export async function sqsReceiveMessages() {
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
        <div class="bg-neutral-900 border border-neutral-800 rounded-lg p-3 relative group">
            <div class="flex justify-between items-center mb-2">
              <div class="">
                <div onclick="openCellModal('Message ID', decodeURIComponent('${m.MessageId}'))" 
                     class="text-[10px] text-gray-300 font-mono mb-1 cursor-pointer hover:text-orange-400 transition-colors" title="Click to expand">
                     ID: ${m.MessageId}
                </div>
                <div onclick="openCellModal('Message MD5', decodeURIComponent('${m.MD5OfBody}'))" 
                      class="text-[10px] text-gray-300 font-mono cursor-pointer hover:text-orange-400 transition-colors" title="Click to expand">
                      MD5: ${m.MD5OfBody}
                </div>
              </div>
              <button onclick="sqsDeleteMessage('${handle}')" 
                      class="text-red-500 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity" 
                      title="Delete Message">
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                              d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                </button>
            </div>
            <div class="text-[10px] text-gray-300 font-mono mb-1">Data:</div>
            <pre class="text-[11px] text-orange-200 overflow-x-auto whitespace-pre-wrap">${m.Body}</pre>
        </div>
    `
  }).join('');
}

export async function sqsSendMessage() {
  const body = document.getElementById('sqs-send-body').value;
  await fetch(
      '/dashboard/api/sqs/send',
      {
        method: 'POST',
        body: JSON.stringify({QueueUrl: sqsCurrentQueueUrl, MessageBody: body})
      }
  );
  document.getElementById('sqs-send-body').value = '';
  await sqsLoadQueues(); // Refresh counts
}

export async function sqsDeleteMessage(encodedHandle) {
  if (!confirm("Delete this specific message?")) {
    return;
  }

  try {
    const res = await fetch(
        `/dashboard/api/sqs/delete-message?url=${encodeURIComponent(sqsCurrentQueueUrl)}&handle=${encodedHandle}`,
        {method: 'POST'}
    );

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