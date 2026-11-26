// Offline educational replay: fixed fictional temperature measurement records.
// No gateway, agent protocol, task execution, or network interfaces.
export type ReadingEvent = Readonly<{
  id: string; minute: number; station: string; temperature: number | null;
}>;
export type Recording = Readonly<{
  id: string; title: string; description: string; events: readonly ReadingEvent[];
}>;
export type StationSummary = {
  station: string; valid: number; missing: number; rejected: number;
  average: number | null; last: number | null;
};
export type ReplayFrame = {
  position: number; events: readonly ReadingEvent[]; stations: StationSummary[];
  duplicates: number; valid: number; missing: number; rejected: number;
};

// Fresh fixtures on every call. A fixed sequence represents observations in
// arrival order, not an operational simulation or a real device connection.
export function recordings(): Recording[] {
  return [
    { id: "baseline", title: "Stable measurements", description: "Two fictional classroom temperature stations.", events: [
      { id: "a1", minute: 0, station: "Classroom A", temperature: 21 },
      { id: "b1", minute: 0, station: "Classroom B", temperature: 22 },
      { id: "a2", minute: 5, station: "Classroom A", temperature: 22 },
      { id: "b2", minute: 5, station: "Classroom B", temperature: 23 },
      { id: "a3", minute: 10, station: "Classroom A", temperature: 23 },
      { id: "b3", minute: 10, station: "Classroom B", temperature: 22 },
    ] },
    { id: "quality", title: "Measurement quality issues", description: "A missing reading, duplicate record and implausible temperature.", events: [
      { id: "a1", minute: 0, station: "Classroom A", temperature: 21 },
      { id: "b1", minute: 0, station: "Classroom B", temperature: 22 },
      { id: "a2", minute: 5, station: "Classroom A", temperature: null },
      { id: "b2", minute: 5, station: "Classroom B", temperature: 23 },
      { id: "b2", minute: 5, station: "Classroom B", temperature: 23 },
      { id: "a3", minute: 10, station: "Classroom A", temperature: 999 },
      { id: "b3", minute: 10, station: "Classroom B", temperature: 24 },
      { id: "a4", minute: 15, station: "Classroom A", temperature: 22 },
    ] },
  ];
}

export function replay(recording: Recording, position: number): ReplayFrame {
  if (!Number.isInteger(position) || position < 0 || position > recording.events.length) {
    throw new RangeError("Invalid replay position");
  }
  // Recompute from the prefix so backward seeks cannot retain future state.
  const events = recording.events.slice(0, position);
  const seen = new Set<string>();
  const stations = new Map<string, StationSummary & { sum: number }>();
  let duplicates = 0;
  for (const event of events) {
    if (seen.has(event.id)) { duplicates++; continue; }
    seen.add(event.id);
    let station = stations.get(event.station);
    if (!station) {
      station = { station: event.station, valid: 0, missing: 0, rejected: 0, average: null, last: null, sum: 0 };
      stations.set(event.station, station);
    }
    const value = event.temperature;
    if (value === null) station.missing++;
    else if (!Number.isFinite(value) || value < -40 || value > 60) station.rejected++;
    else {
      station.valid++;
      station.sum += value;
      station.average = station.sum / station.valid;
      station.last = value;
    }
  }
  const summaries = [...stations.values()].map(({ sum: _sum, ...summary }) => summary);
  return { position, events, stations: summaries, duplicates,
    valid: summaries.reduce((n, s) => n + s.valid, 0),
    missing: summaries.reduce((n, s) => n + s.missing, 0),
    rejected: summaries.reduce((n, s) => n + s.rejected, 0),
  };
}
