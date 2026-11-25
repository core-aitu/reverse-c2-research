// Closed catalog of built-in, browser-local classroom measurement reports.
// No registration of external code, server hooks, network or execution APIs.
export const pluginCatalog = [
  { id: 'quality', title: 'Data quality', version: '1.0.0', description: 'Counts missing, rejected and repeated observations.' },
  { id: 'coverage', title: 'Station coverage', version: '1.0.0', description: 'Counts accepted observations per fictional classroom.' },
  { id: 'temperature', title: 'Temperature summary', version: '1.0.0', description: 'Shows accepted averages and last recorded readings.' },
] as const;
export type PluginId = typeof pluginCatalog[number]['id'];
