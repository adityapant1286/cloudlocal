
export function openCellModal(columnName, value) {
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

export function closeCellModal() {
  document.getElementById('cell-modal').classList.add('hidden');
}

export function copyCellContent() {
  const content = document.getElementById('cell-modal-content').innerText;
  navigator.clipboard.writeText(content);

  const btn = event.target;
  const originalText = btn.innerText;
  btn.innerText = "Copied!";
  setTimeout(() => btn.innerText = originalText, 2000);
}

export function styleSelectedElement(e, selector) {
  document.querySelectorAll(selector)
  .forEach(b => b.classList.remove('bg-neutral-800', 'border-neutral-700', 'text-white'));
  e.classList.add('bg-neutral-800', 'border-neutral-700', 'text-white');
}

export function copyToClipboard(elementId) {
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

export function toISODateFormat(epoch) {
  return new Date(epoch).toISOString();
}