import { useEffect, useState, type ComponentType } from "react";

type TapeEvent = {
  id: string;
  type: string;
  source: string;
  payload: string;
  occurredAt: string;
};

function freshId() {
  return "evt-" + Date.now().toString(36);
}

function stamp(iso: string) {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toISOString().slice(11, 23);
}

export default function Console() {
  const [eventId, setEventId] = useState(freshId);
  const [type, setType] = useState("page.view");
  const [source, setSource] = useState("web");
  const [payload, setPayload] = useState('{"path":"/"}');
  const [status, setStatus] = useState("");
  const [kind, setKind] = useState("");
  const [tick, setTick] = useState(0);
  const [lastId, setLastId] = useState("");
  const [events, setEvents] = useState<TapeEvent[]>([]);
  const [Flow, setFlow] = useState<ComponentType<{ tick: number; kind: string }> | null>(null);

  async function loadTape() {
    const response = await fetch("/query", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        query: "{ events(limit: 12) { id type source payload occurredAt } }",
      }),
    });
    const body = await response.json();
    setEvents(body.data && body.data.events ? body.data.events : []);
  }

  useEffect(() => {
    import("./Pipeline").then((mod) => setFlow(() => mod.default));
    loadTape();
    const timer = window.setInterval(loadTape, 1500);
    return () => window.clearInterval(timer);
  }, []);

  async function send(id: string) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(payload);
    } catch {
      setStatus("Payload has to be JSON.");
      setKind("");
      return;
    }
    const response = await fetch("/v1/events", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        id,
        type: type.trim(),
        source: source.trim(),
        payload: parsed,
        occurredAt: new Date().toISOString(),
      }),
    });
    const body = await response.json().catch(() => ({}));
    if (response.status === 202) {
      setLastId(id);
      setStatus("Accepted " + id);
      setKind("accepted");
      setTick((value) => value + 1);
      setEventId(freshId());
    } else if (body.status === "duplicate") {
      setStatus("Duplicate " + id);
      setKind("duplicate");
      setTick((value) => value + 1);
    } else {
      setStatus(body.error || "Send failed.");
      setKind("");
    }
    await loadTape();
  }

  return (
    <>
      {Flow ? <Flow tick={tick} kind={kind} /> : <div className="flow-slot" />}
      <div className="desk">
        <form
          id="send"
          onSubmit={(event) => {
            event.preventDefault();
            send(eventId.trim());
          }}
        >
          <h2>Send</h2>
          <label>
            Type
            <input name="type" value={type} autoComplete="off" required onChange={(event) => setType(event.target.value)} />
          </label>
          <label>
            Source
            <input name="source" value={source} autoComplete="off" required onChange={(event) => setSource(event.target.value)} />
          </label>
          <label>
            Id
            <input id="event-id" name="id" value={eventId} autoComplete="off" required onChange={(event) => setEventId(event.target.value)} />
          </label>
          <label>
            Payload
            <textarea name="payload" rows={3} required value={payload} onChange={(event) => setPayload(event.target.value)} />
          </label>
          <div className="actions">
            <button type="submit">Send event</button>
            <button
              type="button"
              id="repeat"
              onClick={() => {
                if (!lastId) {
                  setStatus("Send an event first.");
                  return;
                }
                setEventId(lastId);
                send(lastId);
              }}
            >
              Send same id
            </button>
          </div>
          <p id="status" role="status">{status}</p>
        </form>
        <section className="tape" aria-live="polite">
          <h2>Tape</h2>
          <p id="count">{events.length} on the tape</p>
          {events.length === 0 ? (
            <p className="empty">Nothing on the tape.</p>
          ) : (
            <ol id="tape">
              {events.map((event, index) => (
                <li key={event.id} className={index === 0 ? "newest" : undefined}>
                  <span>{stamp(event.occurredAt)}</span>
                  <span>{event.type}</span>
                  <span>{event.id}</span>
                  <span className="payload">{event.source} {event.payload}</span>
                </li>
              ))}
            </ol>
          )}
        </section>
      </div>
    </>
  );
}
