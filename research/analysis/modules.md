# Modules — статическая спецификация 22 managed-файлов

Статус: **static-complete**. Серверный `ModuleManagerService` загружает `modules/*.dll`, экспортирует типы-наследники `Cmdlet` с `[Cmdlet]` и разрешает зависимости по `AssemblyRef`. В 22 файлах найдено **191** экспортируемых PSCmdlet в 16 carriers; ещё 6 файлов — dependency-only.

Роль `product-module-carrier` означает роль в поставке, а не утверждение о юридическом авторстве каждого вложенного фрагмента. `third-party-module-carrier` сохраняет явную внешнюю product/namespace identity; `dependency-library` не экспортирует PSCmdlet.

## Сводная матрица

| Файл | Роль | TypeDef / MethodDef | Cmdlets | P/Invoke | Системные эффекты |
|---|---|---:|---:|---:|---|
| `AD.dll` | product-module-carrier | 406 / 3196 | 32 | 41 | Операции с Kerberos/AD tickets, LDAP/domain-запросы, enrollment сертификатов и credential material. |
| `common.dll` | product-module-carrier | 199 / 630 | 12 | 111 | Обзор файлов/процессов, screenshots, разбор PE, timestomp и приостановка потоков/процессов. |
| `CredentialManager.dll` | third-party-module-carrier | 8 / 19 | 4 | 0 | Чтение, запись и удаление Windows Credential Manager; генерация паролей. |
| `credentials.dll` | product-module-carrier | 357 / 2060 | 21 | 50 | Доступ к credentials/tokens/browser/SAM, keylogging, UAC bypass и запуск процессов с tokens. |
| `Direct.dll` | product-module-carrier | 115 / 413 | 22 | 1 | Прямые NT syscalls для процессов, потоков, sections и virtual memory; shellcode injection. |
| `Dumper.dll` | product-module-carrier | 254 / 637 | 2 | 66 | Получение process/minidump, извлечение shadow copy и разбор credential material. |
| `EventLog.dll` | product-module-carrier | 20 / 107 | 4 | 6 | Запрос Windows Event Log и lifecycle live subscriptions. |
| `Firewall.dll` | product-module-carrier | 41 / 188 | 9 | 33 | Создание, перечисление и удаление правил Windows Firewall и WFP filters. |
| `GPO.dll` | product-module-carrier | 48 / 361 | 21 | 4 | Создание и изменение AD/GPO, links, security filters, scheduled tasks и policies. |
| `Interop.NetFwTypeLib.dll` | dependency-library | 29 / 264 | 0 | 0 | Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface. |
| `Interop.TaskScheduler.dll` | dependency-library | 63 / 545 | 0 | 0 | Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface. |
| `inveigh.dll` | third-party-module-carrier | 198 / 1953 | 7 | 0 | LLMNR/NBNS/mDNS/DNS/DHCPv6 spoofing и захват SMB/HTTP/LDAP credentials. |
| `lateral.dll` | product-module-carrier | 169 / 1070 | 25 | 77 | Remote execution, services/tasks/DCOM, process injection, registry и TCP redirectors. |
| `levelup.dll` | third-party-module-carrier | 151 / 870 | 3 | 232 | Поиск и выполнение техник локального privilege escalation. |
| `Microsoft.GroupPolicy.Management.dll` | dependency-library | 61 / 597 | 0 | 0 | Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface. |
| `persistence.dll` | product-module-carrier | 14 / 56 | 5 | 3 | Изменения persistence через registry, IFEO, COM и ClickOnce. |
| `PSCredentialManager.Api.dll` | dependency-library | 9 / 25 | 0 | 5 | Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface. |
| `PSCredentialManager.Common.dll` | dependency-library | 7 / 4 | 0 | 0 | Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface. |
| `reach.dll` | third-party-module-carrier | 23 / 156 | 3 | 6 | Поиск и выполнение техник remote exploitation. |
| `recon.dll` | product-module-carrier | 47 / 360 | 18 | 0 | Host/domain/network/file/WMI reconnaissance и scanners. |
| `smb.dll` | product-module-carrier | 46 / 217 | 3 | 0 | SMB exploitation и PsExec-style remote execution. |
| `System.DirectoryServices.AccountManagement.dll` | dependency-library | 30 / 281 | 0 | 0 | Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface. |

## Пофайловая поверхность

### `AD.dll`

- Identity: SHA-256 `012b5fa1889c2099338f7799739898bac2ab10306fcd68eda7ab3060c8140bb2`, MVID `480d2bad-171c-4b72-b90e-466798d1c23c`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (32): `Invoke-RubeusASREP2Kirbi`, `Invoke-RubeusAsktgs`, `Invoke-RubeusAsktgt`, `Invoke-RubeusAsktgtfast`, `Invoke-RubeusAsreproast`, `Invoke-RubeusBrute`, `Invoke-RubeusCertenroll`, `Invoke-RubeusChangepw`, `Invoke-RubeusCreatenetonly`, `Invoke-RubeusCurrentluid`, `Invoke-RubeusDescribe`, `Invoke-RubeusDiamond`, `Invoke-RubeusDump`, `Invoke-RubeusGmsapassword`, `Invoke-RubeusGolden`, `Invoke-RubeusHarvest`, `Invoke-RubeusHash`, `Invoke-RubeusKerberoast`, `Invoke-RubeusKirbi`, `Invoke-RubeusKlist`, `Invoke-RubeusLogonsession`, `Invoke-RubeusMonitor`, `Invoke-RubeusPreauthscan`, `Invoke-RubeusPtt`, `Invoke-RubeusPurge`, `Invoke-RubeusRenew`, `Invoke-RubeusS4u`, `Invoke-RubeusShadowcred`, `Invoke-RubeusSilver`, `Invoke-RubeusTgssub`, `Invoke-RubeusTgtdeleg`, `Invoke-RubeusTriage`.
- Входы: `AES128:string`, `AES256:string`, `AES:SwitchParameter`, `Action:string`, `AdditionalSIDs:string`, `AltService:string`, `Armor:string`, `ArmorKey:string`, `AsRep:string`, `AsRepKey:string`, `AuthData:SwitchParameter`, `AuthTime:string`, `AutoEnterprise:SwitchParameter`, `AutoRenew:SwitchParameter`, `BadPwdCount:short`, `BronzeBit:SwitchParameter`, `CA:string`, `CName:string`, `CRealm:string`, `Certificate:string`, `ChangePW:SwitchParameter`, `CreateNetOnly:string`, `CredPassword:string`, `CredUser:string`, `Current:SwitchParameter`, `DC:string`, `DES:SwitchParameter`, `DES:string`, `Delay:int`, `DesPlainText:string`, `DeviceId:string`, `DisplayInterval:int`, `DisplayName:string`, `Dmsa:SwitchParameter`, `Domain:string`, `EncType:string`, `EndTime:string`, `Enterprise:SwitchParameter`, `ExtendedUpnDns:SwitchParameter`, `FilterUser:string`, `Force:SwitchParameter`, `Format:string`, `GetCredentials:SwitchParameter`, `Groups:string`, `HomeDir:string`, `HomeDrive:string`, `Id:int`, `ImpersonateDomain:string`, `ImpersonateUser:string`, `Interval:int`, `Jitter:int`, `Key:string`, `KeyHex:string`, `KeyList:SwitchParameter`, `Kirbi:string`, `KrbEncType:string`, `KrbKey:string`, `LastLogon:string`, `Ldap:SwitchParameter`, `LdapFilter:string`, `LdapPassword:string`, `LdapUser:string`, `Ldaps:SwitchParameter`, `LogoffTime:string`, `LogonCount:short`, `Luid:string`, `MaxPassAge:int`, `MinPassAge:int`, `MonitorInterval:int`, `MsdsSPN:string`, `NTLM:string`, `Netbios:string`, `NewPac:SwitchParameter`, `NewPassword:string`, `NoFullPacSig:SwitchParameter`, `NoPAC:SwitchParameter`, `NoPreAuth:SwitchParameter`, `NoPreAuth:string`, `NoTicket:SwitchParameter`, `NoWrap:SwitchParameter`, `OU:string`, `OldPac:SwitchParameter`, `OldSam:string`, `Opsec:SwitchParameter`, `Outfile:string`, `PGid:int`, `Password:string`, `Passwords:string`, `PrincipalType:string`, `PrintArgs:SwitchParameter`, `PrintCmd:SwitchParameter`, `ProfilePath:string`, `Program:string`, `ProxyUrl:string`, `Ptt:SwitchParameter`, `PwdLastSet:string`, `PwdSetAfter:string`, `PwdSetBefore:string`, `RC4:string`, `RC4Opsec:SwitchParameter`, `RangeEnd:string`, `RangeInterval:string`, `Registry:string`, `RenewTill:string`, `ResourceGroupSid:string`, `ResourceGroups:string`, `ResultLimit:int`, `RodcNumber:int`, `RunFor:int`, `S4uProxyTarget:string`, `S4uTransitedServices:string`, `SID:string`, `SPN:string`, `SPNs:string`, `SRealm:string`, `ScriptPath:string`, `Self:SwitchParameter`, `Server:string`, `Service:string`, `ServiceDomain:string`, `ServiceKey:string`, `ServiceType:string`, `ServiceUser:string`, `SessionEType:string`, `SessionKey:string`, `Show:SwitchParameter`, `Simple:SwitchParameter`, `StartTime:string`, `Stats:SwitchParameter`, `SuppEncType:string`, `TGS:string`, `TGSTicket:string`, `Target:string`, `TargetDC:string`, `TargetDomain:string`, `TargetUser:string`, `Template:string`, `TgtDeleg:SwitchParameter`, `Ticket:string`, `TicketFlags:string`, `TicketUser:string`, `TicketUserID:string`, `U2U:SwitchParameter`, `UAC:string`, `UPN:string`, `UseTGTDeleg:SwitchParameter`, `User:string`, `UserName:string`, `Users:string`, `VerboseOutput:SwitchParameter`, `VerifyCerts:SwitchParameter`, `Wordlist:string[]`.
- Результаты: declared OutputType `RubeusASRepRoastResult`, `RubeusAdcsResult`, `RubeusBruteResult`, `RubeusChangepwResult`, `RubeusCreateNetonlyResult`, `RubeusCurrentLuid`, `RubeusFastResult`, `RubeusForgedTicketResult`, `RubeusGmsaResult`, `RubeusHashResult`, `RubeusKerberoastResult`, `RubeusLogonSession`, `RubeusPreauthscanResult`, `RubeusShadowCredResult`, `RubeusTGTResult`, `RubeusTicketInfo`; каналы записи WriteError у 32 cmdlet, WriteObject у 7 cmdlet.
- P/Invoke (41): модули `CRYPT32.DLL` (1), `Netapi32.dll` (2), `Secur32.dll` (3), `advapi32.dll` (11), `crypt32.dll` (3), `cryptdll.Dll` (2), `kernel32.dll` (7), `mpr.dll` (2), `ntdll.dll` (1), `secur32.dll` (9); полный список entries находится в `managed-semantics.json`.
- Системные эффекты: Операции с Kerberos/AD tickets, LDAP/domain-запросы, enrollment сертификатов и credential material.
- AssemblyRef: `System`, `System.Core`, `System.DirectoryServices`, `System.DirectoryServices.AccountManagement`, `System.DirectoryServices.Protocols`, `System.IdentityModel`, `System.Management.Automation`, `System.Security`, `mscorlib`.

### `common.dll`

- Identity: SHA-256 `9a3f0b7727247962c18981e2924f59ff6b01ff6a6ddf2fd74d3129f5b098bb26`, MVID `6cdcb623-1d54-4c50-861c-3933d650d0e6`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (12): `Get-DosDevicePaths`, `Get-DownloadFile`, `Get-FilesByExtension`, `Get-FilterDrivers`, `Get-FullPath`, `Get-MemoryRegions`, `Get-PEFile`, `Get-Screenshot`, `Invoke-TimeStomp`, `Survey-Processes`, `Suspend-AllThreads`, `Suspend-Process`.
- Входы: `ActiveWindow:SwitchParameter`, `CompressionLevel:int`, `Current:SwitchParameter`, `Detailed:SwitchParameter`, `Extensions:string[]`, `FileInfo:FileInfo`, `FileName:string`, `Filename:string`, `Filepath:string`, `Fullscreen:SwitchParameter`, `MaxResults:int`, `MemoryRegions:MemoryRegion[]`, `PID:int`, `Path:string`, `Process:Process`, `ProcessInfo:ProcessInfoBasic`, `ProcessName:string`, `Raw:SwitchParameter`, `Recurse:SwitchParameter`, `Recurse:bool`, `Source:string`, `Sync:SwitchParameter`, `Target:string`, `TimeStamp:DateTime`.
- Результаты: declared OutputType `FilterDriverInfo`; каналы записи WriteObject у 10 cmdlet, WriteError у 3 cmdlet.
- P/Invoke (111): модули `IpHlpApi.dll` (2), `Netapi32.dll` (4), `advapi32` (1), `advapi32.dll` (43), `fltlib.dll` (1), `kernel32.dll` (33), `mpr.dll` (2), `ntdll.dll` (10), `ole32.dll` (3), `psapi.dll` (1), `shlwapi.dll` (2), `user32.dll` (8), `userenv.dll` (1); полный список entries находится в `managed-semantics.json`.
- Системные эффекты: Обзор файлов/процессов, screenshots, разбор PE, timestomp и приостановка потоков/процессов.
- AssemblyRef: `Interop.TaskScheduler`, `SpecterInsight`, `System`, `System.Drawing`, `System.Management`, `System.Management.Automation`, `System.Windows.Forms`, `mscorlib`.

### `CredentialManager.dll`

- Identity: SHA-256 `694a871f5a3bdf1151cfadc965394e8f30fc1fe6fc96361062df20b4e7f022d1`, MVID `ff113cd6-b80a-45c7-8879-3a371ea1bc98`; роль `third-party-module-carrier`. Экспортирует PSCmdlet-типы; в поставленной сборке сохранена явная third-party product/namespace identity.
- Экспортируемые функции (4): `Get-StoredCredential`, `Get-StrongPassword`, `New-StoredCredential`, `Remove-StoredCredential`.
- Входы: `AsCredentialObject:SwitchParameter`, `Comment:string`, `Credentials:PSCredential`, `Length:int`, `NumberOfSpecialCharacters:int`, `Password:string`, `Persist:CredPersist`, `SecurePassword:SecureString`, `Target:string`, `Type:CredType`, `UserName:string`.
- Результаты: declared OutputType `Credential`, `PSCredential`, `string`; каналы записи WriteObject у 3 cmdlet, WriteError у 3 cmdlet.
- P/Invoke (0): модули —; entries —.
- Системные эффекты: Чтение, запись и удаление Windows Credential Manager; генерация паролей.
- AssemblyRef: `PSCredentialManager.Api`, `PSCredentialManager.Common`, `System`, `System.Core`, `System.Management.Automation`, `System.Web`, `mscorlib`.

### `credentials.dll`

- Identity: SHA-256 `7582cabd4291d45dfafdf1125fe79d4dab07e3076994d7505aa167c60da0cec6`, MVID `66e2123c-8084-4777-8a01-8c0c8d98cf12`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (21): `Clear-CapturedKeys`, `Disable-TokenPrivilege`, `Enable-TokenPrivilege`, `Get-AutologinCredentials`, `Get-BrowserCredentials`, `Get-CapturedKeys`, `Get-Credentials`, `Get-System`, `Get-Token`, `Get-UacBypassTechnique`, `Invoke-LoginPrompt`, `Invoke-PasswordSpray`, `Invoke-UacBypass`, `Invoke-VssSamDump`, `Set-Token`, `Start-KeyLogger`, `Start-ProcessWithToken`, `Steal-Token`, `Stop-KeyLogger`, `Survey-PrivilegeEscalation`, `Survey-UacBypass`.
- Входы: `All:SwitchParameter`, `Arguments:string`, `Browser:string`, `Caption:string`, `Command:string`, `CreateSnapshot:bool`, `Current:SwitchParameter`, `DcIp:string`, `DelaySeconds:int`, `Disk:string`, `Domain:string`, `ExcludeDisabled:SwitchParameter`, `ExcludePotentialLockouts:SwitchParameter`, `Filepath:string`, `Force:SwitchParameter`, `ForceTopmost:SwitchParameter`, `JitterSeconds:int`, `LdapFilter:string`, `Message:string`, `Method:CredentialEdtractionMethod`, `Name:string`, `NoValidation:SwitchParameter`, `ObservationWindowMinutes:int`, `OutsideDomain:SwitchParameter`, `PID:uint`, `Password:string`, `Password:string[]`, `Privileges:PrivilegeName[]`, `Process:Process`, `ProcessInfo:ProcessInfoBasic`, `Script:string`, `SuccessOnly:SwitchParameter`, `Technique:GetSystemTechnique`, `Technique:UacBypassTechnique`, `Token:TokenVault.Token`, `Type:PrivilegeEscalationSurvey`, `User:string[]`, `UserListFile:string`, `VulnerableOnly:SwitchParameter`, `WorkingDirectory:string`.
- Результаты: declared OutputType `LoginPromptResult`, `PasswordSprayResult`; каналы записи WriteObject у 14 cmdlet.
- P/Invoke (50): модули `bcrypt.dll` (6), `credui.dll` (2), `crypt32.dll` (1), `kernel32.dll` (4), `ntdll.dll` (9), `ole32.dll` (4), `oleaut32.dll` (7), `shell32.dll` (1), `user32.dll` (16); полный список entries находится в `managed-semantics.json`.
- Системные эффекты: Доступ к credentials/tokens/browser/SAM, keylogging, UAC bypass и запуск процессов с tokens.
- AssemblyRef: `SpecterInsight`, `System`, `System.DirectoryServices`, `System.Management`, `System.Management.Automation`, `System.ServiceProcess`, `System.Windows.Forms`, `common`, `mscorlib`.

### `Direct.dll`

- Identity: SHA-256 `c30f24cfceac505e226a671138a9d75610b03d161ee323b21020307c2db1fbfa`, MVID `2aaf66a0-c515-4135-aa2a-a996756ebd84`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (22): `Close-HandleSyscalls`, `Get-ThreadContextSyscalls`, `Get-VirtualMemoryInfoSyscalls`, `Inject-ShellcodeSyscalls`, `Invoke-MapSectionSyscalls`, `Invoke-UnmapSectionSyscalls`, `Migrate-ProcessSyscalls`, `New-SectionSyscalls`, `New-ThreadSyscalls`, `New-VirtualMemorySyscalls`, `Open-ProcessSyscalls`, `Open-ProcessTokenSyscalls`, `Open-ThreadSyscalls`, `Read-VirtualMemorySyscalls`, `Remove-VirtualMemorySyscalls`, `Resume-ProcessSyscalls`, `Resume-ThreadSyscalls`, `Set-MemoryProtectionSyscalls`, `Set-ThreadContextSyscalls`, `Suspend-ProcessSyscalls`, `Suspend-ThreadSyscalls`, `Write-VirtualMemorySyscalls`.
- Входы: `Access:ThreadAccess`, `Access:Win32.Kernel32.ProcessAccessFlags`, `Access:int`, `Access:uint`, `Address:IntPtr`, `AllocationAttributes:uint`, `AllocationType:AllocationType`, `Args:Hashtable`, `BaseAddress:IntPtr`, `Build:string`, `Context:CONTEXT`, `Data:byte[]`, `Handle:IntPtr`, `InheritDisposition:uint`, `Name:string`, `PID:int`, `PageProtection:uint`, `Parameter:IntPtr`, `Pipeline:string`, `Process:Process`, `ProcessHandle:IntPtr`, `ProcessInfo:ProcessInfoBasic`, `ProcessName:string`, `Protection:MemoryProtectionConstants`, `Protection:uint`, `SectionHandle:IntPtr`, `Shellcode:byte[]`, `Size:int`, `Size:long`, `StartAddress:IntPtr`, `Suspended:SwitchParameter`, `TID:int`, `Technique:string`, `Timeout:TimeSpan`.
- Результаты: declared OutputType —; каналы записи WriteObject у 16 cmdlet.
- P/Invoke (1): модули `kernel32.dll` (1); entries `kernel32.dll!VirtualQueryEx`.
- Системные эффекты: Прямые NT syscalls для процессов, потоков, sections и virtual memory; shellcode injection.
- AssemblyRef: `SpecterInsight`, `System`, `System.Management.Automation`, `mscorlib`.

### `Dumper.dll`

- Identity: SHA-256 `a9e0fc749eda0ed29e6074926cb2ef07d6864ebe1367d8a66cd0ef6a66e344a0`, MVID `112f73f1-e6dc-4f15-bfab-35122fa6bc66`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (2): `Invoke-ShadowExtract`, `New-ShadowDump`.
- Входы: `Compress:SwitchParameter`, `Encrypt:SwitchParameter`, `OutputPath:string`, `Technique:DumpTechnique`, `Technique:ExtractTechnique`.
- Результаты: declared OutputType `FileInfo`, `LogonSession`, `PSObject`; каналы записи WriteObject у 2 cmdlet.
- P/Invoke (66): модули `advapi32` (1), `advapi32.dll` (6), `bcrypt` (6), `dbghelp.dll` (1), `kernel32.dll` (35), `ntdll.dll` (17); полный список entries находится в `managed-semantics.json`.
- Системные эффекты: Получение process/minidump, извлечение shadow copy и разбор credential material.
- AssemblyRef: `System`, `System.Management.Automation`, `mscorlib`.

### `EventLog.dll`

- Identity: SHA-256 `094531ab90bf68cd100d50f95340d24b7aeccc604df76cfff1109b180a08a823`, MVID `a5eae887-5e67-4ca1-8aee-470cd3797e31`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (4): `Get-EventSubscription`, `Get-Events`, `Start-EventSubscription`, `Stop-EventSubscription`.
- Входы: `All:SwitchParameter`, `ComputerName:string`, `Filters:Hashtable`, `Id:long`, `Logname:string`, `Max:int`, `Password:string`, `Query:string`, `ScriptBlock:ScriptBlock`, `Subscription:EventSubscription`, `Timeout:int`, `Username:string`.
- Результаты: declared OutputType —; каналы записи WriteObject у 2 cmdlet.
- P/Invoke (6): модули `wevtapi.dll` (6); entries `wevtapi.dll!EvtClose`, `wevtapi.dll!EvtNext`, `wevtapi.dll!EvtOpenSession`, `wevtapi.dll!EvtQuery`, `wevtapi.dll!EvtRender`, `wevtapi.dll!EvtSubscribe`.
- Системные эффекты: Запрос Windows Event Log и lifecycle live subscriptions.
- AssemblyRef: `SpecterInsight`, `System`, `System.Management.Automation`, `System.Xml`, `common`, `mscorlib`.

### `Firewall.dll`

- Identity: SHA-256 `2de374ab0116ae1a8f32c274dcbd876acf0ab4ed47bb5e03e4a61d63b026f9ba`, MVID `f91ea26a-2dc9-492f-bdc3-b9f1792e307a`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (9): `FwRule-Apply`, `FwRule-New`, `FwRule-Remove`, `WfpBlock-Add`, `WfpBlock-Get`, `WfpBlock-Remove`, `WfpFilter-Remove`, `WfpIp-Block`, `WfpProcess-Block`.
- Входы: `Action:FirewallAction`, `Address:string`, `Description:string`, `Direction:FirewallDirection`, `ExecutablePath:string`, `FilterId:ulong`, `LHosts:string[]`, `LPorts:string[]`, `Name:string`, `Path:string`, `Protocol:FirewallProtocol`, `RHosts:string[]`, `RPorts:string[]`, `Rule:FwRule`.
- Результаты: declared OutputType —; каналы записи WriteObject у 7 cmdlet.
- P/Invoke (33): модули `advapi32.dll` (7), `fwpuclnt.dll` (13), `kernel32.dll` (13); полный список entries находится в `managed-semantics.json`.
- Системные эффекты: Создание, перечисление и удаление правил Windows Firewall и WFP filters.
- AssemblyRef: `Interop.NetFwTypeLib`, `System`, `System.Management.Automation`, `common`, `mscorlib`.

### `GPO.dll`

- Identity: SHA-256 `bb600a86192f23cba4f3e91b2658454c652f4979e224dd41a85b9ab685c0886a`, MVID `29c3e0d0-1dbc-4afe-9902-a57be6908edc`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (21): `Ad-GetDomain`, `Ad-GetDomainComputer`, `Ad-GetDomainUser`, `GpLink-Add`, `GpLink-Get`, `GpLink-Remove`, `Gpo-FwRuleAdd`, `Gpo-Get`, `Gpo-ImmediateTaskAdd`, `Gpo-New`, `Gpo-NewSession`, `Gpo-Remove`, `Gpo-SecurityFilterAdd`, `Gpo-SecurityFilterGet`, `Gpo-SecurityFilterRemove`, `Gpo-VersionIncrement`, `Ou-Get`, `Ou-GetParent`, `Ou-MoveTo`, `Ou-New`, `Ou-Remove`.
- Входы: `All:SwitchParameter`, `Arguments:string`, `Author:string`, `Computer:string`, `ComputerFilter:string`, `Domain:string`, `DomainController:string`, `Entry:DirectoryEntry`, `ExecutablePath:string`, `GUID:string`, `GpoSession:GpoManagerDirect`, `Group:string`, `Hidden:bool`, `MultipleInstancesPolicy:MultipleInstancesPolicy`, `NTAccount:string`, `Name:string`, `OU:DirectoryEntry`, `Object:DirectoryEntry`, `OrganizationalUnit:DirectoryEntry`, `Parent:DirectoryEntry`, `Password:string`, `Policy:DirectoryEntry`, `Priority:ScheduledTaskPriority`, `Rule:FwRule`, `RunAsUser:string`, `TaskDescription:string`, `TaskName:string`, `User:string`, `Username:string`, `WakeToRun:bool`.
- Результаты: declared OutputType —; каналы записи WriteObject у 10 cmdlet.
- P/Invoke (4): модули `advapi32.dll` (3), `kernel32.dll` (1); entries `advapi32.dll!ImpersonateLoggedOnUser`, `advapi32.dll!LogonUser`, `advapi32.dll!RevertToSelf`, `kernel32.dll!CloseHandle`.
- Системные эффекты: Создание и изменение AD/GPO, links, security filters, scheduled tasks и policies.
- AssemblyRef: `Firewall`, `System`, `System.Core`, `System.DirectoryServices`, `System.DirectoryServices.AccountManagement`, `System.Management.Automation`, `System.Xml`, `common`, `mscorlib`.

### `Interop.NetFwTypeLib.dll`

- Identity: SHA-256 `e13a32101c5c456048e555697ad91a1fd5792c804dbe7d399c57ee885de42c2f`, MVID `038bf1bc-d0af-424f-bd76-35c062b4d6e4`; роль `dependency-library`. Нет экспортируемых PSCmdlet-типов; сборка используется module carriers как зависимость.
- Экспортируемые функции (0): —.
- Входы: —.
- Результаты: declared OutputType —; каналы записи —.
- P/Invoke (0): модули —; entries —.
- Системные эффекты: Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface.
- AssemblyRef: `mscorlib`.

### `Interop.TaskScheduler.dll`

- Identity: SHA-256 `6317b1cdbc554624184f58597fe2ea4d87733de0985a918204d3135959a6cbac`, MVID `75d0ba55-f5d4-457a-86cd-d9b385a3a030`; роль `dependency-library`. Нет экспортируемых PSCmdlet-типов; сборка используется module carriers как зависимость.
- Экспортируемые функции (0): —.
- Входы: —.
- Результаты: declared OutputType —; каналы записи —.
- P/Invoke (0): модули —; entries —.
- Системные эффекты: Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface.
- AssemblyRef: `mscorlib`.

### `inveigh.dll`

- Identity: SHA-256 `f119919c3c5979a205ee4569580976e9fac37a9922eb31e349f2500acdafbcdf`, MVID `dd68d0b6-7c86-498b-8b34-b6272cc685c9`; роль `third-party-module-carrier`. Экспортирует PSCmdlet-типы; в поставленной сборке сохранена явная third-party product/namespace identity.
- Экспортируемые функции (7): `Clear-Inveigh`, `Get-InveighCredential`, `Get-InveighStatus`, `Start-InveighListener`, `Start-InveighSpoofer`, `Stop-Inveigh`, `Wait-InveighCredential`.
- Входы: `Cert:string`, `CertPassword:string`, `Challenge:string`, `ComputerName:string`, `Count:int`, `DNSDomain:string`, `DNSIPv6:string`, `DNSSuffix:string`, `EnableWebDAV:SwitchParameter`, `HTTPPort:int`, `HTTPSPort:int`, `Hashcat:SwitchParameter`, `IPAddress:string`, `IgnoreDomains:string[]`, `IgnoreIPs:string[]`, `IgnoreQueries:string[]`, `Inspect:SwitchParameter`, `LDAPPort:int`, `MAC:string`, `NetbiosDomain:string`, `Protocol:string[]`, `ProxyPort:int`, `Repeat:SwitchParameter`, `ReplyIP:string`, `ReplyIPv6:string`, `ReplyToDomains:string[]`, `ReplyToIPs:string[]`, `ReplyToQueries:string[]`, `RunTime:int`, `SMBPort:int`, `TTL:uint`, `TimeoutSeconds:int`, `Unique:SwitchParameter`, `WPADResponse:string`.
- Результаты: declared OutputType `CleartextCredential`, `ListenerStatus`, `NTLMCredential`, `string`; каналы записи WriteObject у 6 cmdlet.
- P/Invoke (0): модули —; entries —.
- Системные эффекты: LLMNR/NBNS/mDNS/DNS/DHCPv6 spoofing и захват SMB/HTTP/LDAP credentials.
- AssemblyRef: `System`, `System.Core`, `System.DirectoryServices.Protocols`, `System.Management.Automation`, `mscorlib`.

### `lateral.dll`

- Identity: SHA-256 `f877efafec959d731f88057dd06b981ee9482985ec21d72e7eb52cab8c67e76c`, MVID `1ff7e0fb-6336-400a-8b7d-b7ce98773160`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (25): `Add-PortProxy`, `Create-Service`, `Get-CsaTargets`, `Get-PortProxy`, `Get-RegKeyValue`, `Get-RemoteCredentials`, `Get-TcpRedirector`, `Get-TcpRedirectorSession`, `Inject-Shellcode`, `Invoke-CrossSessionActivation`, `Invoke-Dcom`, `Invoke-InMemoryPE`, `Invoke-ParallelCommand`, `Invoke-ScheduledTask`, `Migrate-Process`, `New-GhostTask`, `Remove-GhostTask`, `Remove-PortProxy`, `Remove-RegKeyValue`, `Remove-Service`, `Set-RegKeyValue`, `Spawn-Inject`, `Start-TcpRedirector`, `Stop-TcpRedirector`, `Stop-TcpRedirectorSession`.
- Входы: `Allocator:string`, `Application:string`, `Args:Hashtable`, `Argument:string`, `ArgumentList:string[]`, `Arguments:string`, `Build:string`, `Bytes:byte[]`, `Capture:SwitchParameter`, `Command:string`, `ComputerName:string`, `ComputerName:string[]`, `ConnectAddress:string`, `ConnectPort:int`, `DayOfWeek:GhostTaskDayOfWeek`, `DestinationAddress:IPAddress`, `DestinationPort:int`, `Directory:string`, `Family:string`, `Hive:Advapi32.RemoteRegistryKeyHive`, `Hour:int`, `Interval:int`, `Key:string`, `ListenAddress:string`, `ListenPort:int`, `Method:CsaMethod`, `Method:Dcom.DcomMethod`, `Methods:CommandRunnerMethod[]`, `Minute:int`, `Name:string`, `NoStart:SwitchParameter`, `PID:int`, `Password:string`, `Path:string`, `Pipeline:string`, `ProcessName:string`, `Program:string`, `Redirector:TcpRedirectorService`, `RunAsSystem:SwitchParameter`, `ScheduleType:ScheduleType`, `ServiceName:string`, `Session:TcpRedirectorSession`, `SessionId:uint`, `Shellcode:byte[]`, `SourceAddress:IPAddress`, `SourcePort:int`, `SpawnTarget:string`, `System:string`, `Targets:string[]`, `TaskName:string`, `TaskPath:string`, `Technique:string`, `Threads:uint`, `Timeout:TimeSpan`, `Timeout:int`, `Upsert:bool`, `UserName:string`, `Username:string`, `Value:object`.
- Результаты: declared OutputType `ComCsaEntry`, `CsaResult`, `DcomResult`; каналы записи WriteObject у 16 cmdlet, WriteError у 4 cmdlet.
- P/Invoke (77): модули `advapi32.dll` (12), `kernel32.dll` (54), `kernelbase.dll` (2), `mpr.dll` (2), `ntdll.dll` (7); полный список entries находится в `managed-semantics.json`.
- Системные эффекты: Remote execution, services/tasks/DCOM, process injection, registry и TCP redirectors.
- AssemblyRef: `Interop.TaskScheduler`, `SpecterInsight`, `System`, `System.Management`, `System.Management.Automation`, `common`, `mscorlib`.

### `levelup.dll`

- Identity: SHA-256 `9f75f8de371d26e5143dc254109a80434072bb1811a34e305e793d6c930754b9`, MVID `40f5436a-6828-4c54-bdf7-653577796c5c`; роль `third-party-module-carrier`. Экспортирует PSCmdlet-типы; в поставленной сборке сохранена явная third-party product/namespace identity.
- Экспортируемые функции (3): `Get-LevelUpSupportedTechniques`, `Invoke-LevelUp`, `Invoke-LevelUpRunCommand`.
- Входы: `BuildId:string`, `Command:string`, `DllPath:string`, `IncludeAll:SwitchParameter`, `Technique:string`.
- Результаты: declared OutputType —; каналы записи WriteObject у 3 cmdlet.
- P/Invoke (232): модули `ProjectedFSLib.dll` (1), `advapi32.dll` (35), `cldapi.dll` (5), `clfsw32.dll` (3), `combase.dll` (6), `faultrep.dll` (1), `fltlib.dll` (2), `gdi32.dll` (4), `kernel32.dll` (73), `mpr.dll` (2), `netapi32.dll` (2), `ntdll.dll` (25), `ole32.dll` (14), `rpcrt4.dll` (19), `secur32.dll` (11), `shell32.dll` (2), `user32.dll` (11), `virtdisk.dll` (4), `wer.dll` (5), `win32u.dll` (1), `winspool.drv` (6); полный список entries находится в `managed-semantics.json`.
- Системные эффекты: Поиск и выполнение техник локального privilege escalation.
- AssemblyRef: `System`, `System.Management.Automation`, `mscorlib`.

### `Microsoft.GroupPolicy.Management.dll`

- Identity: SHA-256 `9fd9e20b06253d040d6e2ae3afe930a013fc4199fc844d6543a54c1c49c60331`, MVID `5de59291-aa2f-4a1c-832e-d75a8c61af47`; роль `dependency-library`. Нет экспортируемых PSCmdlet-типов; сборка используется module carriers как зависимость.
- Экспортируемые функции (0): —.
- Входы: —.
- Результаты: declared OutputType —; каналы записи —.
- P/Invoke (0): модули —; entries —.
- Системные эффекты: Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface.
- AssemblyRef: `Microsoft.GroupPolicy.Management.Interop`, `Microsoft.GroupPolicy.ServerAdminTools.GpmgmtLib`, `System`, `System.DirectoryServices`, `System.Management`, `System.Xml`, `mscorlib`.

### `persistence.dll`

- Identity: SHA-256 `230ae977abba3f0d1188ed79405cd33771e6eba2acacabcf56c200dfd59c2b9c`, MVID `42388b96-475e-472b-ae9d-ac4f278bba2c`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (5): `Set-ClickOnceComHijackPersistence`, `Set-ComDllHijackPersistence`, `Set-HiddenRunPersistence`, `Set-IfeoHijackPersistence`, `Set-RegistryPersistence`.
- Входы: `Clsid:string`, `Command:string`, `DllPath:string`, `Enumerate:bool`, `Name:string`, `Remove:bool`, `TargetProcess:string`, `Type:RegistryKeyType`.
- Результаты: declared OutputType —; каналы записи WriteObject у 2 cmdlet.
- P/Invoke (3): модули `ntdll.dll` (3); entries `ntdll.dll!NtClose`, `ntdll.dll!NtOpenKey`, `ntdll.dll!NtSetValueKey`.
- Системные эффекты: Изменения persistence через registry, IFEO, COM и ClickOnce.
- AssemblyRef: `System.Management.Automation`, `mscorlib`.

### `PSCredentialManager.Api.dll`

- Identity: SHA-256 `43ca6c1c63cf8177b67b526f39352ed9c2c56fadbe6a227de0fc27df0c36e4c9`, MVID `8140e985-fa0b-4b61-9a71-2248b42b5089`; роль `dependency-library`. Нет экспортируемых PSCmdlet-типов; сборка используется module carriers как зависимость.
- Экспортируемые функции (0): —.
- Входы: —.
- Результаты: declared OutputType —; каналы записи —.
- P/Invoke (5): модули `Advapi32.dll` (5); entries `Advapi32.dll!CredDeleteW`, `Advapi32.dll!CredEnumerateW`, `Advapi32.dll!CredFree`, `Advapi32.dll!CredReadW`, `Advapi32.dll!CredWriteW`.
- Системные эффекты: Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface.
- AssemblyRef: `PSCredentialManager.Common`, `System.Core`, `mscorlib`.

### `PSCredentialManager.Common.dll`

- Identity: SHA-256 `ade4d3af0d0437eb9f00b1c594c8dc1a528a0dd41fc1c1f19034181455bd44fc`, MVID `552273e3-6962-487b-9dcd-05c7d68a8846`; роль `dependency-library`. Нет экспортируемых PSCmdlet-типов; сборка используется module carriers как зависимость.
- Экспортируемые функции (0): —.
- Входы: —.
- Результаты: declared OutputType —; каналы записи —.
- P/Invoke (0): модули —; entries —.
- Системные эффекты: Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface.
- AssemblyRef: `mscorlib`.

### `reach.dll`

- Identity: SHA-256 `5e481dc2ebc2a6476e0f778f86f44943075a3920c157c0f0728332f8c4df45d1`, MVID `4d326663-55b6-4436-9ce9-f039ed1ff029`; роль `third-party-module-carrier`. Экспортирует PSCmdlet-типы; в поставленной сборке сохранена явная third-party product/namespace identity.
- Экспортируемые функции (3): `Get-SupportedExploits`, `Invoke-Reach`, `Invoke-ReachRunCommand`.
- Входы: `BuildId:string`, `Command:string`, `Domain:string`, `Exploit:string`, `GadgetBytes:byte[]`, `Kernel32Base:ulong`, `Kernel32FilePath:string`, `NtdllBase:ulong`, `NtdllFilePath:string`, `Port:int`, `ShellcodeBytes:byte[]`, `Target:string`.
- Результаты: declared OutputType —; каналы записи WriteObject у 3 cmdlet.
- P/Invoke (6): модули `advapi32.dll` (4), `kernel32.dll` (2); entries `kernel32.dll!CloseHandle`, `kernel32.dll!GetCurrentProcess`, `advapi32.dll!GetTokenInformation`, `advapi32.dll!OpenProcessToken`, `advapi32.dll!RegCloseKey`, `advapi32.dll!RegOpenKeyEx`.
- Системные эффекты: Поиск и выполнение техник remote exploitation.
- AssemblyRef: `System`, `System.Management.Automation`, `mscorlib`.

### `recon.dll`

- Identity: SHA-256 `c1079fd338765e3842a5f95b9fc66406d2eb7fc4126b4ab8e8d8920f010f67d9`, MVID `bbd5cc4d-b74c-46ce-9e98-ccc69f283126`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (18): `Get-Antivirus`, `Get-Computers`, `Get-DomainGroupMembers`, `Get-DomainUsers`, `Get-Files`, `Get-Interfaces`, `Get-LocalGroups`, `Get-LocalUsers`, `Get-RecentFiles`, `Get-SystemInfo`, `Get-TcpConnections`, `Get-TcpListeners`, `Get-WmiEventFilter`, `Invoke-PingSweep`, `Invoke-ResolveIP`, `Invoke-SmbScanner`, `Invoke-TcpScanner`, `Invoke-WmiScanner`.
- Входы: `ComputerName:string`, `Domain:string`, `Extended:SwitchParameter`, `Filters:Regex[]`, `Group:string`, `Hostnames:SwitchParameter`, `IncludeShellBags:SwitchParameter`, `MaxItems:int`, `OU:string`, `Password:string`, `Path:string[]`, `Port:int`, `Ports:int[]`, `Randomize:SwitchParameter`, `Recurse:SwitchParameter`, `ResolveNames:SwitchParameter`, `ResolveUncHosts:SwitchParameter`, `Server:string`, `Target:string[]`, `Targets:IPAddress[]`, `Targets:object[]`, `Targets:string[]`, `ThreadCount:int`, `Threads:uint`, `Timeout:int`, `TimeoutMs:int`, `Username:string`.
- Результаты: declared OutputType —; каналы записи WriteObject у 18 cmdlet, WriteError у 4 cmdlet.
- P/Invoke (0): модули —; entries —.
- Системные эффекты: Host/domain/network/file/WMI reconnaissance и scanners.
- AssemblyRef: `SpecterInsight`, `System`, `System.DirectoryServices`, `System.Management`, `System.Management.Automation`, `System.Windows.Forms`, `common`, `mscorlib`.

### `smb.dll`

- Identity: SHA-256 `b73c2d2a1912f4bad56055bae21f1ba95a51a96363ac86c38eadd6caece7e9d1`, MVID `f234a594-5745-4c31-8690-1593d3458a49`; роль `product-module-carrier`. Экспортирует PSCmdlet-типы под identity модуля поставки; внутри может присутствовать upstream-derived code.
- Экспортируемые функции (3): `Invoke-EternalBlue`, `Invoke-PsExec`, `Invoke-SMBGhost`.
- Входы: `AdminCheck:bool`, `BuildId:string`, `ComSpec:bool`, `Command:string`, `ComputerName:string`, `ComputerName:string[]`, `Domain:string`, `ForceSMBv1:bool`, `Hash:string`, `Password:string`, `Port:int`, `Randomize:SwitchParameter`, `ServiceName:string`, `Shellcode:byte[]`, `Sleep:int`, `Target:string[]`, `Threads:uint`, `TimeoutMs:int`, `TriggerCrash:bool`, `Username:string`.
- Результаты: declared OutputType `EternalBlueResult`, `SmbGhostResult`; каналы записи WriteObject у 2 cmdlet.
- P/Invoke (0): модули —; entries —.
- Системные эффекты: SMB exploitation и PsExec-style remote execution.
- AssemblyRef: `SpecterInsight`, `System`, `System.Management.Automation`, `common`, `mscorlib`.

### `System.DirectoryServices.AccountManagement.dll`

- Identity: SHA-256 `b813b45bdf7c56fcd260d231e1eb05a2c4c0a9b365b029779df3d4355e5aa48d`, MVID `0f9e7aa2-3dc9-4d18-8df2-060a6761d387`; роль `dependency-library`. Нет экспортируемых PSCmdlet-типов; сборка используется module carriers как зависимость.
- Экспортируемые функции (0): —.
- Входы: —.
- Результаты: declared OutputType —; каналы записи —.
- P/Invoke (0): модули —; entries —.
- Системные эффекты: Dependency types/interop declarations; эффекты возникают через callers, а не через экспортируемую cmdlet surface.
- AssemblyRef: `System`, `System.Configuration`, `System.DirectoryServices`, `System.DirectoryServices.Protocols`, `System.Xml`, `mscorlib`.

Полные сигнатуры входных параметров каждого cmdlet, declared output types и все P/Invoke entries: [managed-semantics.json](managed-semantics.json).

Граница результата: эффекты выведены из статических путей и API/import surface; фактические изменения конкретной системы не наблюдались.
