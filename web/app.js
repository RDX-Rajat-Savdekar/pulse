const form = document.querySelector("#send");
const idInput = document.querySelector("#event-id");
const statusEl = document.querySelector("#status");
const tape = document.querySelector("#tape");
const countEl = document.querySelector("#count");

let lastId = "";
let lastMarkup = "";

function freshId() {
  return "evt-" + Date.now().toString(36);
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[ch]));
}

function stamp(iso) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toISOString().slice(11, 23);
}

idInput.value = freshId();

async function loadTape() {
  const response = await fetch("/query", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      query: "{ events(limit: 20) { id type source payload occurredAt } }",
    }),
  });
  const body = await response.json();
  const events = body.data && body.data.events ? body.data.events : [];
  const nextCount = events.length + " on the tape";
  if (countEl.textContent !== nextCount) countEl.textContent = nextCount;
  const markup = events.length === 0
    ? '<p class="empty">Nothing on the tape.</p>'
    : events.map((event) => `
    <li>
      <span>${escapeHtml(stamp(event.occurredAt))}</span>
      <span>${escapeHtml(event.type)}</span>
      <span>${escapeHtml(event.id)}</span>
      <span class="payload">${escapeHtml(event.source)} ${escapeHtml(event.payload)}</span>
    </li>
  `).join("");
  if (markup !== lastMarkup) {
    tape.innerHTML = markup;
    lastMarkup = markup;
  }
}

async function send(keepId) {
  const data = new FormData(form);
  if (keepId && lastId) idInput.value = lastId;
  const id = idInput.value.trim();
  let payload;
  try {
    payload = JSON.parse(String(data.get("payload")));
  } catch {
    statusEl.textContent = "Payload has to be JSON.";
    return;
  }
  const response = await fetch("/v1/events", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      id,
      type: String(data.get("type")).trim(),
      source: String(data.get("source")).trim(),
      payload,
      occurredAt: new Date().toISOString(),
    }),
  });
  const body = await response.json().catch(() => ({}));
  if (response.status === 202) {
    lastId = id;
    statusEl.textContent = "Accepted " + id;
    idInput.value = freshId();
  } else if (body.status === "duplicate") {
    statusEl.textContent = "Duplicate " + id;
  } else {
    statusEl.textContent = body.error || "Send failed.";
  }
  await loadTape();
}

form.addEventListener("submit", (event) => {
  event.preventDefault();
  send(false);
});

document.querySelector("#repeat").addEventListener("click", () => {
  if (!lastId) {
    statusEl.textContent = "Send an event first.";
    return;
  }
  send(true);
});

loadTape();
setInterval(loadTape, 1500);
