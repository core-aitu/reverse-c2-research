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

export type RealAgent = {
  id: string;
  name: string;
  source: "real";
  protocolVersion: number;
  buildVersion: string;
  capabilities: string[];
  architecture: string;
  os: string;
  generation: number;
  enabled: boolean;
  revokedAt?: string;
  firstSeen: string;
  lastCheckin: string;
  nextExpectedCheckin: string;
  health: "online" | "degraded" | "offline";
  intervalMs: number;
  lastSequence: number;
  lastCursor: number;
};

export type GatewayListener = {
  endpoint: string;
  address: string;
  tlsConfigured: boolean;
  ready: boolean;
  status: "online" | "degraded" | "offline";
};

export type GatewayTaskView = {
  id: string;
  agentId: string;
  correlationId: string;
  version: number;
  idempotencyKey: string;
  kind: "echo" | "system-info" | "sleep" | "exec" | "shell" | "file-download";
  status: "queued" | "leased" | "running" | "completed" | "failed" | "cancelled";
  leaseUntil?: string;
  deadline: string;
  timeoutMs: number;
  cancelRequestedAt?: string;
  startedAt?: string;
  dispatchCount: number;
  retries: number;
  error?: { code: string; message: string; retryable: boolean };
  createdAt: string;
  completedAt?: string;
};

export type ObjectInfo = {
  id: string;
  kind: string;
  name: string;
  sha256: string;
  mediaType: string;
  size: number;
  createdAt: string;
};

export type CommandResult = {
  exitCode?: number;
  durationMs?: number;
  stdoutBytes?: number;
  stderrBytes?: number;
  truncated?: boolean;
  stdoutObjectId?: string;
  stderrObjectId?: string;
  objectId?: string;
  object?: ObjectInfo;
};

export type GatewayTaskDetail = GatewayTaskView & {
  payload?: Record<string, unknown>;
  result?: CommandResult;
  outputs?: Array<{ stream: "stdout" | "stderr"; object: ObjectInfo; byteCount: number; truncated: boolean }>;
};

export type TaskOutputChunk = {
  taskId: string;
  stream: "stdout" | "stderr";
  index: number;
  sequence?: number;
  text?: string;
  data?: string;
  encoding?: "utf8" | "base64";
  eof?: boolean;
  truncated?: boolean;
};

export type TaskOutputEnvelope = {
  id?: number;
  type?: string;
  payload?: TaskOutputChunk;
} & Partial<TaskOutputChunk>;

export type BeaconRelease = {
  id: string;
  version: string;
  os: "linux" | "windows" | "darwin";
  architecture: "amd64" | "arm64";
  sha256?: string;
  size?: number;
  signed: boolean;
  signatureStatus?: "missing" | "present";
  development?: boolean;
  object?: ObjectInfo;
  sbom?: ObjectInfo;
  checksums?: ObjectInfo;
  signature?: ObjectInfo;
  createdAt: string;
};

export type BeaconBuild = {
  id: string;
  releaseId: string;
  version: string;
  os: string;
  architecture: string;
  sha256?: string;
  createdBy?: string;
  lastBundleAt?: string;
  createdAt: string;
};

export type EnrollmentTokenInfo = {
  id: string;
  expiresAt: string;
  usedAt?: string;
  createdBy: string;
  createdAt: string;
};

export type EnrollmentTokenCreated = EnrollmentTokenInfo & { token: string };

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
export type ArtifactContentInfo = ArtifactInfo & { sha256: string; mediaType: string };
export type SerializableVersion = { major: number; minor: number; build: number; revision: number };
export type UpdateJobInfo = { version: SerializableVersion; status: string; statusDescription: string; lastUpdateResult: string; progress: number };
export type UpdatePackageInfo = { version: SerializableVersion; sha256: string; mediaType: string; length: number; active: boolean; createdAt: string };
export type LicenseValidation = { key: string; success: boolean; license: string; checksum: string; expires: string; customer_email: string };
export type CapabilityDefinition = { id: string; group: string; title: string; state: "registered" | "fixture" | "implemented" | "runtime-verified" | "evidence-blocked"; routes: string[]; evidence?: string };
export type NetHost = { hostId: string; hostname: string; domain: string; osName: string; osVersion: string; osArchitecture: string; cidrNetwork: string; firstSeen: string; lastSeen: string };
export type NetHostIpAddress = { ipId: string; hostId: string; ipAddress: string; isPrimary: boolean; interfaceName: string; macAddress: string; vendor: string };
