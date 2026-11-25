export type Identity = {
  username: string;
  roles: string[];
};

export type AuthResponse = {
  username: string;
  enabled: boolean;
  roles: string[];
  jwtToken: string;
  expiresAt: string;
};

export type DocumentationEntry = {
  name: string;
  contents: string;
  children: DocumentationEntry[];
};

export type SearchResult = {
  path: string;
  title: string;
  content: string;
  score: number;
};

export type AuditEvent = {
  id: number;
  identifier: string;
  timestamp: string;
  type: string;
  source: string;
  outcome: string;
  system: string;
  message: string;
  details: string;
};

export type EventEnvelope = {
  id: number;
  type: string;
  occurredAt: string;
  version: number;
  payload: AuditEvent;
};

export type Setting = { key: string; value: string };

export type Account = {
  username: string;
  enabled: boolean;
  roles: string[];
};

export type Resource = {
  kind: string;
  key: string;
  name: string;
  description: string;
  status: string;
  spec: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
};

export type Adapter = {
  name: string;
  category: string;
  mode: "fixture" | "real";
  enabled: boolean;
  endpoint: string;
  status: string;
  lastError: string;
  checkedAt: string | null;
  config: Record<string, unknown>;
  updatedAt: string;
};

export type Job = {
  id: string;
  kind: string;
  status: "queued" | "running" | "succeeded" | "failed" | "cancelled";
  input: Record<string, unknown>;
  output?: Record<string, unknown>;
  error: string;
  attempt: number;
  maxAttempts: number;
  cancelRequested: boolean;
  requestedBy: string;
  createdAt: string;
  startedAt: string | null;
  completedAt: string | null;
};

export type PipelineDefinition = {
  id: string;
  name: string;
  description: string;
  definition: {
    version: number;
    nodes: Array<{ id: string; type: string; dependsOn: string[]; value?: string }>;
    outputs: string[];
  };
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
};

export type AgentSession = {
  id: string;
  name: string;
  transport: "simulator";
  status: "online" | "offline" | "archived";
  metadata: Record<string, unknown>;
  firstSeen: string;
  lastCheckin: string;
};

export type AgentTask = {
  id: string;
  sessionId: string;
  kind: "echo" | "inventory";
  payload: Record<string, unknown>;
  status: "queued" | "dispatched" | "completed" | "failed" | "cancelled";
  result?: Record<string, unknown>;
  error: string;
  createdAt: string;
  completedAt: string | null;
};

export type SpecterScript = {
  identifier: string;
  name: string;
  description: string;
  documentation: string;
  scriptBlock: string;
  version: number;
  isPremium: boolean;
  labels: string[];
  enabled: boolean;
};

export type Wordlist = {
  id: number;
  name: string;
  description: string;
  tags: string[];
  content: string;
  entryCount: number;
  createdAt: string;
  updatedAt: string;
};

export type Listener = {
  prefix: string;
  enabled: boolean;
  certificateName: string;
  webDavPath: string;
};

export type CertificateInfo = {
  name: string;
  country: string;
  state: string;
  locality: string;
  organization: string;
  organizationalUnit: string;
  commonName: string;
  start: string;
  end: string;
};

export type SpecterModule = {
  name: string;
  version: string;
  runtimeVersion: string;
  cmdlets: string[];
};

export type NotebookCell = {
  type: "markdown" | "code";
  contents: string;
};

export type PowerShellNotebook = {
  id: number;
  notebookId: string;
  name: string;
  description: string;
  timeCreated: string;
  timeUpdated: string;
  cells: NotebookCell[];
};

export type AiAgentDefinition = {
  id: number;
  agentId: string;
  name: string;
  description: string;
  prompt: string;
  includeSpecterScripts: boolean;
  accessibleFunctions: string[];
};

export type AiFunctionDefinition = {
  functionId: string;
  name: string;
  description: string;
  documentation: string;
  approvalLevel: "Low" | "Medium" | "High";
  definition: string;
};

export type RunAiFunctionResult = { succeeded: boolean; output: string };

export type AiSession = {
  sessionId: string;
  name: string;
  agents: AiAgentDefinition[];
  modelType: string;
  modelName: string;
  toolApprovalPolicy: "AutoApproveAll" | "AutoApproveLow" | "AutoApproveMedium" | "AutoApproveNone";
  plan: Record<string, unknown>;
  status: "Running" | "Pausing" | "Paused" | "Starting" | "Stopped";
};

export type ArtifactInfo = { host: string; name: string; length: number; creationDate: string };
export type SerializableVersion = { major: number; minor: number; build: number; revision: number };
export type UpdateJobInfo = { version: SerializableVersion; status: string; statusDescription: string; lastUpdateResult: string; progress: number };
export type LicenseValidation = { key: string; success: boolean; license: string; checksum: string; expires: string; customer_email: string };
