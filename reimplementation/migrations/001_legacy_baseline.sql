-- Generated from research/validation/stage5/evidence/postgres-catalog.json.
-- This is a clean-room catalog transcription, not decompiled implementation code.
CREATE EXTENSION IF NOT EXISTS "citext";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

CREATE TYPE "aifunctioncallstatus" AS ENUM ('Created', 'PendingApproval', 'Approved', 'Running', 'Failed', 'Denied', 'Complete');
CREATE TYPE "aisendertype" AS ENUM ('User', 'Assistant', 'System', 'Tool');
CREATE TYPE "aisessionstatus" AS ENUM ('Running', 'Pausing', 'Paused', 'Starting', 'Stopped');
CREATE TYPE "implanttaskstatus" AS ENUM ('NotQueued', 'Queued', 'Tasked', 'Completed', 'Cancelled');
CREATE TYPE "sshauthtype" AS ENUM ('Password', 'PrivateKey', 'Certificate');
CREATE TYPE "sshtaskstatus" AS ENUM ('Running', 'Completed', 'Failed', 'Cancelled');
CREATE TYPE "toolcallapprovallevel" AS ENUM ('Low', 'Medium', 'High');
CREATE TYPE "toolcallapprovalpolicy" AS ENUM ('AutoApproveAll', 'AutoApproveLow', 'AutoApproveMedium', 'AutoApproveNone');
CREATE TYPE "winrmauthtype" AS ENUM ('Password', 'PassTheHash', 'Kerberos', 'Certificate');
CREATE TYPE "winrmtaskstatus" AS ENUM ('Running', 'Completed', 'Failed', 'Cancelled');

CREATE SEQUENCE account_id_seq;
CREATE SEQUENCE acmeaccount_id_seq;
CREATE SEQUENCE aiagentdefinitions_id_seq;
CREATE SEQUENCE aifunctioncallresults_id_seq;
CREATE SEQUENCE aifunctiondefinitions_id_seq;
CREATE SEQUENCE aimessageartifacts_id_seq;
CREATE SEQUENCE aimessages_id_seq;
CREATE SEQUENCE aisessions_id_seq;
CREATE SEQUENCE artifacts_id_seq;
CREATE SEQUENCE documentationchunks_id_seq;
CREATE SEQUENCE emailaccount_id_seq;
CREATE SEQUENCE event_id_seq;
CREATE SEQUENCE implanttasks_id_seq;
CREATE SEQUENCE label_id_seq;
CREATE SEQUENCE listener_id_seq;
CREATE SEQUENCE netcachedfiles_id_seq;
CREATE SEQUENCE netcredentials_id_seq;
CREATE SEQUENCE nethostipaddresses_id_seq;
CREATE SEQUENCE nethosts_id_seq;
CREATE SEQUENCE netopenports_id_seq;
CREATE SEQUENCE netprocesses_id_seq;
CREATE SEQUENCE netservices_id_seq;
CREATE SEQUENCE netusers_id_seq;
CREATE SEQUENCE payloadpipelines_id_seq;
CREATE SEQUENCE persona_id_seq;
CREATE SEQUENCE powershellnotebooks_id_seq;
CREATE SEQUENCE refreshtokens_id_seq;
CREATE SEQUENCE script_id_seq;
CREATE SEQUENCE scriptlabel_id_seq;
CREATE SEQUENCE session_id_seq;
CREATE SEQUENCE wordlist_id_seq;

CREATE TABLE "account" (
  "id" integer DEFAULT nextval('account_id_seq'::regclass) NOT NULL,
  "username" character varying(32) NOT NULL,
  "hash" character(60) NOT NULL,
  "enabled" boolean NOT NULL
);
CREATE TABLE "acmeaccount" (
  "id" integer DEFAULT nextval('acmeaccount_id_seq'::regclass) NOT NULL,
  "email" text NOT NULL,
  "certificate" text NOT NULL
);
CREATE TABLE "aiagentdefinitions" (
  "id" integer DEFAULT nextval('aiagentdefinitions_id_seq'::regclass) NOT NULL,
  "agentid" text NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL,
  "prompt" text NOT NULL,
  "includespecterscripts" boolean NOT NULL
);
CREATE TABLE "aiagentfunctionlinks" (
  "agentid" integer NOT NULL,
  "functionid" integer NOT NULL
);
CREATE TABLE "aifunctioncallresults" (
  "id" integer DEFAULT nextval('aifunctioncallresults_id_seq'::regclass) NOT NULL,
  "sessionid" integer NOT NULL,
  "toolcallid" text NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL,
  "argumentsjson" text NOT NULL,
  "output" text NOT NULL,
  "error" text,
  "executedutc" timestamp with time zone NOT NULL,
  "approvalstatus" aifunctioncallstatus NOT NULL
);
CREATE TABLE "aifunctiondefinitions" (
  "id" integer DEFAULT nextval('aifunctiondefinitions_id_seq'::regclass) NOT NULL,
  "funcid" text NOT NULL,
  "name" text NOT NULL,
  "internalname" text NOT NULL,
  "description" text NOT NULL,
  "documentation" text NOT NULL,
  "definition" text NOT NULL,
  "approvallevel" toolcallapprovallevel NOT NULL
);
CREATE TABLE "aimessageartifacts" (
  "id" integer DEFAULT nextval('aimessageartifacts_id_seq'::regclass) NOT NULL,
  "aimessageid" integer NOT NULL,
  "artifactid" integer NOT NULL
);
CREATE TABLE "aimessages" (
  "id" integer DEFAULT nextval('aimessages_id_seq'::regclass) NOT NULL,
  "sessionid" integer NOT NULL,
  "messageid" text NOT NULL,
  "timecreated" timestamp with time zone NOT NULL,
  "timecompleted" timestamp with time zone,
  "sendertype" aisendertype NOT NULL,
  "sendername" text NOT NULL,
  "receivername" text NOT NULL,
  "message" text NOT NULL,
  "summarizedtext" text,
  "status" text NOT NULL
);
CREATE TABLE "aisessions" (
  "id" integer DEFAULT nextval('aisessions_id_seq'::regclass) NOT NULL,
  "sessionid" text NOT NULL,
  "name" text NOT NULL,
  "agents" text NOT NULL,
  "modeltype" text NOT NULL,
  "modelname" text NOT NULL,
  "approvalpolicy" toolcallapprovalpolicy NOT NULL,
  "status" aisessionstatus NOT NULL,
  "plan" text NOT NULL
);
CREATE TABLE "artifacts" (
  "id" integer DEFAULT nextval('artifacts_id_seq'::regclass) NOT NULL,
  "sha256" text NOT NULL,
  "name" text NOT NULL,
  "extension" text,
  "username" text,
  "filesize" bigint NOT NULL,
  "sourcetype" text,
  "source" text,
  "sourcepath" text,
  "createdtimestamp" timestamp with time zone NOT NULL,
  "ingestedtimestamp" timestamp with time zone NOT NULL
);
CREATE TABLE "documentationchunks" (
  "id" integer DEFAULT nextval('documentationchunks_id_seq'::regclass) NOT NULL,
  "path" text NOT NULL,
  "title" text NOT NULL,
  "content" text NOT NULL,
  "embedding" real[],
  "chunkindex" integer DEFAULT 0 NOT NULL,
  "indexedat" timestamp with time zone DEFAULT now() NOT NULL
);
CREATE TABLE "emailaccount" (
  "id" integer DEFAULT nextval('emailaccount_id_seq'::regclass) NOT NULL,
  "personaid" integer,
  "provider" text DEFAULT ''::text NOT NULL,
  "emailaddress" text NOT NULL,
  "username" text DEFAULT ''::text NOT NULL,
  "password" text DEFAULT ''::text NOT NULL,
  "imaphost" text DEFAULT ''::text NOT NULL,
  "imapport" integer DEFAULT 993 NOT NULL,
  "imapssl" text DEFAULT 'SslOnConnect'::text NOT NULL,
  "smtphost" text DEFAULT ''::text NOT NULL,
  "smtpport" integer DEFAULT 465 NOT NULL,
  "smtpssl" text DEFAULT 'SslOnConnect'::text NOT NULL,
  "active" boolean DEFAULT true NOT NULL,
  "notes" text DEFAULT ''::text NOT NULL,
  "createdat" timestamp with time zone DEFAULT now() NOT NULL,
  "updatedat" timestamp with time zone DEFAULT now() NOT NULL
);
CREATE TABLE "event" (
  "id" integer DEFAULT nextval('event_id_seq'::regclass) NOT NULL,
  "identifier" text NOT NULL,
  "timestamp" timestamp with time zone NOT NULL,
  "type" text NOT NULL,
  "source" text NOT NULL,
  "outcome" text NOT NULL,
  "system" text NOT NULL,
  "message" text NOT NULL,
  "details" text NOT NULL
);
CREATE TABLE "implanttasks" (
  "id" integer DEFAULT nextval('implanttasks_id_seq'::regclass) NOT NULL,
  "sessionid" integer,
  "taskid" text NOT NULL,
  "scriptblock" text NOT NULL,
  "runinbackground" boolean NOT NULL,
  "textoutputonly" boolean NOT NULL,
  "labels" text[],
  "scriptname" text NOT NULL,
  "scriptdescription" text NOT NULL,
  "status" implanttaskstatus NOT NULL,
  "timequeued" timestamp with time zone NOT NULL,
  "timetasked" timestamp with time zone,
  "timecompleted" timestamp with time zone,
  "results" text,
  "errors" text,
  "log" text
);
CREATE TABLE "label" (
  "id" integer DEFAULT nextval('label_id_seq'::regclass) NOT NULL,
  "name" text NOT NULL
);
CREATE TABLE "listener" (
  "id" integer DEFAULT nextval('listener_id_seq'::regclass) NOT NULL,
  "prefix" text NOT NULL,
  "enabled" boolean NOT NULL,
  "certificatename" text NOT NULL,
  "webdavpath" text
);
CREATE TABLE "netcachedfiles" (
  "id" integer DEFAULT nextval('netcachedfiles_id_seq'::regclass) NOT NULL,
  "fileid" text NOT NULL,
  "hostid" text NOT NULL,
  "path" text NOT NULL,
  "sizebytes" bigint,
  "hashmd5" text,
  "hashsha256" text,
  "content" text,
  "mimetype" text,
  "isdirectory" boolean DEFAULT false NOT NULL,
  "discoveredat" timestamp with time zone NOT NULL
);
CREATE TABLE "netcredentials" (
  "id" integer DEFAULT nextval('netcredentials_id_seq'::regclass) NOT NULL,
  "credentialid" text NOT NULL,
  "hostid" text NOT NULL,
  "credentialtype" smallint NOT NULL,
  "username" text,
  "domain" text,
  "secret" text,
  "additionaldata" text,
  "firstseen" timestamp with time zone NOT NULL,
  "lastseen" timestamp with time zone NOT NULL
);
CREATE TABLE "nethostipaddresses" (
  "id" integer DEFAULT nextval('nethostipaddresses_id_seq'::regclass) NOT NULL,
  "ipid" text NOT NULL,
  "hostid" text NOT NULL,
  "ipaddress" text NOT NULL,
  "isprimary" boolean DEFAULT false NOT NULL,
  "interfacename" text,
  "macaddress" text
);
CREATE TABLE "nethosts" (
  "id" integer DEFAULT nextval('nethosts_id_seq'::regclass) NOT NULL,
  "hostid" text NOT NULL,
  "hostname" text,
  "domain" text,
  "osname" text,
  "osversion" text,
  "osarchitecture" text,
  "cidrnetwork" text,
  "firstseen" timestamp with time zone NOT NULL,
  "lastseen" timestamp with time zone NOT NULL
);
CREATE TABLE "netopenports" (
  "id" integer DEFAULT nextval('netopenports_id_seq'::regclass) NOT NULL,
  "portid" text NOT NULL,
  "hostid" text NOT NULL,
  "portnumber" integer NOT NULL,
  "protocol" text NOT NULL,
  "state" text NOT NULL,
  "servicename" text,
  "banner" text,
  "firstseen" timestamp with time zone NOT NULL,
  "lastseen" timestamp with time zone NOT NULL
);
CREATE TABLE "netprocesses" (
  "id" integer DEFAULT nextval('netprocesses_id_seq'::regclass) NOT NULL,
  "processrecordid" text NOT NULL,
  "hostid" text NOT NULL,
  "pid" integer NOT NULL,
  "name" text NOT NULL,
  "executablepath" text,
  "commandline" text,
  "username" text,
  "architecture" text,
  "observedat" timestamp with time zone NOT NULL
);
CREATE TABLE "netservices" (
  "id" integer DEFAULT nextval('netservices_id_seq'::regclass) NOT NULL,
  "serviceid" text NOT NULL,
  "hostid" text NOT NULL,
  "name" text NOT NULL,
  "displayname" text,
  "status" text,
  "starttype" text,
  "executablepath" text,
  "firstseen" timestamp with time zone NOT NULL,
  "lastseen" timestamp with time zone NOT NULL
);
CREATE TABLE "netusers" (
  "id" integer DEFAULT nextval('netusers_id_seq'::regclass) NOT NULL,
  "userid" text NOT NULL,
  "hostid" text NOT NULL,
  "username" text NOT NULL,
  "domain" text,
  "sid" text,
  "isenabled" boolean,
  "isadmin" boolean,
  "groups" text,
  "lastlogon" timestamp with time zone,
  "firstseen" timestamp with time zone NOT NULL,
  "lastseen" timestamp with time zone NOT NULL
);
CREATE TABLE "payloadpipelines" (
  "id" integer DEFAULT nextval('payloadpipelines_id_seq'::regclass) NOT NULL,
  "pipelineid" text NOT NULL,
  "name" citext NOT NULL,
  "description" text NOT NULL,
  "documentation" text NOT NULL,
  "scriptblock" text NOT NULL,
  "outputformat" text NOT NULL,
  "ispremium" boolean NOT NULL,
  "isenabled" boolean NOT NULL,
  "testparameters" text NOT NULL
);
CREATE TABLE "persona" (
  "id" integer DEFAULT nextval('persona_id_seq'::regclass) NOT NULL,
  "name" text NOT NULL,
  "country" text DEFAULT ''::text NOT NULL,
  "placeofbirth" text DEFAULT ''::text NOT NULL,
  "address" text DEFAULT ''::text NOT NULL,
  "backstory" text DEFAULT ''::text NOT NULL,
  "properties" jsonb DEFAULT '{}'::jsonb NOT NULL,
  "createdat" timestamp with time zone DEFAULT now() NOT NULL,
  "updatedat" timestamp with time zone DEFAULT now() NOT NULL
);
CREATE TABLE "powershellnotebooks" (
  "id" integer DEFAULT nextval('powershellnotebooks_id_seq'::regclass) NOT NULL,
  "notebookid" text NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL,
  "timecreated" timestamp with time zone NOT NULL,
  "timeupdated" timestamp with time zone NOT NULL,
  "cells" jsonb NOT NULL
);
CREATE TABLE "refreshtokens" (
  "id" integer DEFAULT nextval('refreshtokens_id_seq'::regclass) NOT NULL,
  "account_id" integer,
  "token" text NOT NULL,
  "created" timestamp with time zone NOT NULL,
  "expires" timestamp with time zone NOT NULL,
  "created_by_ip" text NOT NULL,
  "revoked" boolean NOT NULL
);
CREATE TABLE "script" (
  "id" integer DEFAULT nextval('script_id_seq'::regclass) NOT NULL,
  "scriptid" text NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL,
  "documentation" text NOT NULL,
  "scriptblock" text NOT NULL,
  "version" integer NOT NULL,
  "ispremium" boolean NOT NULL,
  "enabled" boolean NOT NULL
);
CREATE TABLE "scriptlabel" (
  "id" integer DEFAULT nextval('scriptlabel_id_seq'::regclass) NOT NULL,
  "fkey_scriptid" integer,
  "fkey_labelid" integer
);
CREATE TABLE "session" (
  "id" integer DEFAULT nextval('session_id_seq'::regclass) NOT NULL,
  "buildid" text NOT NULL,
  "sessionid" text NOT NULL,
  "nickname" text NOT NULL,
  "machineid" text NOT NULL,
  "fqdn" text NOT NULL,
  "ip" text NOT NULL,
  "path" text NOT NULL,
  "pid" integer NOT NULL,
  "username" text NOT NULL,
  "os" text NOT NULL,
  "architecture" text NOT NULL,
  "lastcheckin" timestamp with time zone NOT NULL,
  "nextcheckin" timestamp with time zone NOT NULL,
  "interval" interval NOT NULL,
  "Window" interval NOT NULL,
  "archived" boolean NOT NULL,
  "expirationdate" timestamp with time zone NOT NULL,
  "notes" text NOT NULL
);
CREATE TABLE "sshsessions" (
  "sessionid" uuid NOT NULL,
  "targethost" text NOT NULL,
  "port" integer NOT NULL,
  "username" text NOT NULL,
  "authtype" sshauthtype NOT NULL,
  "tunnelsessionid" text,
  "createdat" timestamp with time zone NOT NULL,
  "endedat" timestamp with time zone,
  "notes" text
);
CREATE TABLE "sshtasks" (
  "taskid" uuid NOT NULL,
  "sessionid" uuid NOT NULL,
  "command" text NOT NULL,
  "status" sshtaskstatus NOT NULL,
  "timequeued" timestamp with time zone NOT NULL,
  "timecompleted" timestamp with time zone,
  "output" text,
  "errors" text
);
CREATE TABLE "winrmsessions" (
  "sessionid" uuid NOT NULL,
  "targethost" text NOT NULL,
  "port" integer NOT NULL,
  "username" text NOT NULL,
  "authtype" winrmauthtype NOT NULL,
  "usessl" boolean DEFAULT false NOT NULL,
  "useragent" text DEFAULT 'Microsoft WinRM Client'::text NOT NULL,
  "tunnelsessionid" text,
  "createdat" timestamp with time zone NOT NULL,
  "endedat" timestamp with time zone,
  "notes" text
);
CREATE TABLE "winrmtasks" (
  "taskid" uuid NOT NULL,
  "sessionid" uuid NOT NULL,
  "command" text NOT NULL,
  "status" winrmtaskstatus NOT NULL,
  "timequeued" timestamp with time zone NOT NULL,
  "timecompleted" timestamp with time zone,
  "output" text,
  "errors" text
);
CREATE TABLE "wordlist" (
  "id" integer DEFAULT nextval('wordlist_id_seq'::regclass) NOT NULL,
  "name" text NOT NULL,
  "description" text DEFAULT ''::text NOT NULL,
  "tags" text[] DEFAULT '{}'::text[] NOT NULL,
  "content" text,
  "createdat" timestamp with time zone DEFAULT now() NOT NULL,
  "updatedat" timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE "account" ADD CONSTRAINT "account_pkey" PRIMARY KEY (id);
ALTER TABLE "account" ADD CONSTRAINT "account_username_key" UNIQUE (username);
ALTER TABLE "acmeaccount" ADD CONSTRAINT "acmeaccount_email_key" UNIQUE (email);
ALTER TABLE "acmeaccount" ADD CONSTRAINT "acmeaccount_pkey" PRIMARY KEY (id);
ALTER TABLE "aiagentdefinitions" ADD CONSTRAINT "aiagentdefinitions_agentid_key" UNIQUE (agentid);
ALTER TABLE "aiagentdefinitions" ADD CONSTRAINT "aiagentdefinitions_name_key" UNIQUE (name);
ALTER TABLE "aiagentdefinitions" ADD CONSTRAINT "aiagentdefinitions_pkey" PRIMARY KEY (id);
ALTER TABLE "aiagentfunctionlinks" ADD CONSTRAINT "aiagentfunctionlinks_pkey" PRIMARY KEY (agentid, functionid);
ALTER TABLE "aifunctioncallresults" ADD CONSTRAINT "aifunctioncallresults_pkey" PRIMARY KEY (id);
ALTER TABLE "aifunctioncallresults" ADD CONSTRAINT "aifunctioncallresults_toolcallid_key" UNIQUE (toolcallid);
ALTER TABLE "aifunctiondefinitions" ADD CONSTRAINT "aifunctiondefinitions_funcid_key" UNIQUE (funcid);
ALTER TABLE "aifunctiondefinitions" ADD CONSTRAINT "aifunctiondefinitions_internalname_key" UNIQUE (internalname);
ALTER TABLE "aifunctiondefinitions" ADD CONSTRAINT "aifunctiondefinitions_name_key" UNIQUE (name);
ALTER TABLE "aifunctiondefinitions" ADD CONSTRAINT "aifunctiondefinitions_pkey" PRIMARY KEY (id);
ALTER TABLE "aimessageartifacts" ADD CONSTRAINT "aimessageartifacts_aimessageid_artifactid_key" UNIQUE (aimessageid, artifactid);
ALTER TABLE "aimessageartifacts" ADD CONSTRAINT "aimessageartifacts_pkey" PRIMARY KEY (id);
ALTER TABLE "aimessages" ADD CONSTRAINT "aimessages_messageid_key" UNIQUE (messageid);
ALTER TABLE "aimessages" ADD CONSTRAINT "aimessages_pkey" PRIMARY KEY (id);
ALTER TABLE "aisessions" ADD CONSTRAINT "aisessions_name_key" UNIQUE (name);
ALTER TABLE "aisessions" ADD CONSTRAINT "aisessions_pkey" PRIMARY KEY (id);
ALTER TABLE "aisessions" ADD CONSTRAINT "aisessions_sessionid_key" UNIQUE (sessionid);
ALTER TABLE "artifacts" ADD CONSTRAINT "artifacts_pkey" PRIMARY KEY (id);
ALTER TABLE "documentationchunks" ADD CONSTRAINT "documentationchunks_pkey" PRIMARY KEY (id);
ALTER TABLE "emailaccount" ADD CONSTRAINT "emailaccount_emailaddress_key" UNIQUE (emailaddress);
ALTER TABLE "emailaccount" ADD CONSTRAINT "emailaccount_pkey" PRIMARY KEY (id);
ALTER TABLE "event" ADD CONSTRAINT "event_pkey" PRIMARY KEY (id);
ALTER TABLE "implanttasks" ADD CONSTRAINT "implanttasks_pkey" PRIMARY KEY (id);
ALTER TABLE "implanttasks" ADD CONSTRAINT "implanttasks_taskid_key" UNIQUE (taskid);
ALTER TABLE "label" ADD CONSTRAINT "label_name_key" UNIQUE (name);
ALTER TABLE "label" ADD CONSTRAINT "label_pkey" PRIMARY KEY (id);
ALTER TABLE "listener" ADD CONSTRAINT "listener_pkey" PRIMARY KEY (id);
ALTER TABLE "listener" ADD CONSTRAINT "listener_prefix_key" UNIQUE (prefix);
ALTER TABLE "netcachedfiles" ADD CONSTRAINT "netcachedfiles_fileid_key" UNIQUE (fileid);
ALTER TABLE "netcachedfiles" ADD CONSTRAINT "netcachedfiles_hostid_path_key" UNIQUE (hostid, path);
ALTER TABLE "netcachedfiles" ADD CONSTRAINT "netcachedfiles_pkey" PRIMARY KEY (id);
ALTER TABLE "netcredentials" ADD CONSTRAINT "netcredentials_credentialid_key" UNIQUE (credentialid);
ALTER TABLE "netcredentials" ADD CONSTRAINT "netcredentials_pkey" PRIMARY KEY (id);
ALTER TABLE "nethostipaddresses" ADD CONSTRAINT "nethostipaddresses_hostid_ipaddress_key" UNIQUE (hostid, ipaddress);
ALTER TABLE "nethostipaddresses" ADD CONSTRAINT "nethostipaddresses_ipid_key" UNIQUE (ipid);
ALTER TABLE "nethostipaddresses" ADD CONSTRAINT "nethostipaddresses_pkey" PRIMARY KEY (id);
ALTER TABLE "nethosts" ADD CONSTRAINT "nethosts_hostid_key" UNIQUE (hostid);
ALTER TABLE "nethosts" ADD CONSTRAINT "nethosts_pkey" PRIMARY KEY (id);
ALTER TABLE "netopenports" ADD CONSTRAINT "netopenports_hostid_portnumber_protocol_key" UNIQUE (hostid, portnumber, protocol);
ALTER TABLE "netopenports" ADD CONSTRAINT "netopenports_pkey" PRIMARY KEY (id);
ALTER TABLE "netopenports" ADD CONSTRAINT "netopenports_portid_key" UNIQUE (portid);
ALTER TABLE "netprocesses" ADD CONSTRAINT "netprocesses_pkey" PRIMARY KEY (id);
ALTER TABLE "netprocesses" ADD CONSTRAINT "netprocesses_processrecordid_key" UNIQUE (processrecordid);
ALTER TABLE "netservices" ADD CONSTRAINT "netservices_hostid_name_key" UNIQUE (hostid, name);
ALTER TABLE "netservices" ADD CONSTRAINT "netservices_pkey" PRIMARY KEY (id);
ALTER TABLE "netservices" ADD CONSTRAINT "netservices_serviceid_key" UNIQUE (serviceid);
ALTER TABLE "netusers" ADD CONSTRAINT "netusers_pkey" PRIMARY KEY (id);
ALTER TABLE "netusers" ADD CONSTRAINT "netusers_userid_key" UNIQUE (userid);
ALTER TABLE "payloadpipelines" ADD CONSTRAINT "payloadpipelines_name_key" UNIQUE (name);
ALTER TABLE "payloadpipelines" ADD CONSTRAINT "payloadpipelines_pipelineid_key" UNIQUE (pipelineid);
ALTER TABLE "payloadpipelines" ADD CONSTRAINT "payloadpipelines_pkey" PRIMARY KEY (id);
ALTER TABLE "persona" ADD CONSTRAINT "persona_name_key" UNIQUE (name);
ALTER TABLE "persona" ADD CONSTRAINT "persona_pkey" PRIMARY KEY (id);
ALTER TABLE "powershellnotebooks" ADD CONSTRAINT "powershellnotebooks_notebookid_key" UNIQUE (notebookid);
ALTER TABLE "powershellnotebooks" ADD CONSTRAINT "powershellnotebooks_pkey" PRIMARY KEY (id);
ALTER TABLE "refreshtokens" ADD CONSTRAINT "refreshtokens_pkey" PRIMARY KEY (id);
ALTER TABLE "script" ADD CONSTRAINT "script_pkey" PRIMARY KEY (id);
ALTER TABLE "script" ADD CONSTRAINT "script_scriptid_key" UNIQUE (scriptid);
ALTER TABLE "scriptlabel" ADD CONSTRAINT "scriptlabel_pkey" PRIMARY KEY (id);
ALTER TABLE "session" ADD CONSTRAINT "session_pkey" PRIMARY KEY (id);
ALTER TABLE "session" ADD CONSTRAINT "session_sessionid_key" UNIQUE (sessionid);
ALTER TABLE "sshsessions" ADD CONSTRAINT "sshsessions_pkey" PRIMARY KEY (sessionid);
ALTER TABLE "sshtasks" ADD CONSTRAINT "sshtasks_pkey" PRIMARY KEY (taskid);
ALTER TABLE "winrmsessions" ADD CONSTRAINT "winrmsessions_pkey" PRIMARY KEY (sessionid);
ALTER TABLE "winrmtasks" ADD CONSTRAINT "winrmtasks_pkey" PRIMARY KEY (taskid);
ALTER TABLE "wordlist" ADD CONSTRAINT "wordlist_name_key" UNIQUE (name);
ALTER TABLE "wordlist" ADD CONSTRAINT "wordlist_pkey" PRIMARY KEY (id);
ALTER TABLE "aiagentfunctionlinks" ADD CONSTRAINT "aiagentfunctionlinks_agentid_fkey" FOREIGN KEY (agentid) REFERENCES aiagentdefinitions(id) ON DELETE CASCADE;
ALTER TABLE "aiagentfunctionlinks" ADD CONSTRAINT "aiagentfunctionlinks_functionid_fkey" FOREIGN KEY (functionid) REFERENCES aifunctiondefinitions(id) ON DELETE CASCADE;
ALTER TABLE "aifunctioncallresults" ADD CONSTRAINT "aifunctioncallresults_sessionid_fkey" FOREIGN KEY (sessionid) REFERENCES aisessions(id) ON DELETE CASCADE;
ALTER TABLE "aimessageartifacts" ADD CONSTRAINT "aimessageartifacts_aimessageid_fkey" FOREIGN KEY (aimessageid) REFERENCES aimessages(id) ON DELETE CASCADE;
ALTER TABLE "aimessageartifacts" ADD CONSTRAINT "aimessageartifacts_artifactid_fkey" FOREIGN KEY (artifactid) REFERENCES artifacts(id) ON DELETE CASCADE;
ALTER TABLE "aimessages" ADD CONSTRAINT "aimessages_sessionid_fkey" FOREIGN KEY (sessionid) REFERENCES aisessions(id) ON DELETE CASCADE;
ALTER TABLE "emailaccount" ADD CONSTRAINT "emailaccount_personaid_fkey" FOREIGN KEY (personaid) REFERENCES persona(id) ON DELETE SET NULL;
ALTER TABLE "implanttasks" ADD CONSTRAINT "implanttasks_sessionid_fkey" FOREIGN KEY (sessionid) REFERENCES session(id) ON DELETE CASCADE;
ALTER TABLE "netcachedfiles" ADD CONSTRAINT "netcachedfiles_hostid_fkey" FOREIGN KEY (hostid) REFERENCES nethosts(hostid) ON DELETE CASCADE;
ALTER TABLE "netcredentials" ADD CONSTRAINT "netcredentials_hostid_fkey" FOREIGN KEY (hostid) REFERENCES nethosts(hostid) ON DELETE CASCADE;
ALTER TABLE "nethostipaddresses" ADD CONSTRAINT "nethostipaddresses_hostid_fkey" FOREIGN KEY (hostid) REFERENCES nethosts(hostid) ON DELETE CASCADE;
ALTER TABLE "netopenports" ADD CONSTRAINT "netopenports_hostid_fkey" FOREIGN KEY (hostid) REFERENCES nethosts(hostid) ON DELETE CASCADE;
ALTER TABLE "netprocesses" ADD CONSTRAINT "netprocesses_hostid_fkey" FOREIGN KEY (hostid) REFERENCES nethosts(hostid) ON DELETE CASCADE;
ALTER TABLE "netservices" ADD CONSTRAINT "netservices_hostid_fkey" FOREIGN KEY (hostid) REFERENCES nethosts(hostid) ON DELETE CASCADE;
ALTER TABLE "netusers" ADD CONSTRAINT "netusers_hostid_fkey" FOREIGN KEY (hostid) REFERENCES nethosts(hostid) ON DELETE CASCADE;
ALTER TABLE "refreshtokens" ADD CONSTRAINT "refreshtokens_account_id_fkey" FOREIGN KEY (account_id) REFERENCES account(id) ON DELETE CASCADE;
ALTER TABLE "scriptlabel" ADD CONSTRAINT "scriptlabel_fkey_labelid_fkey" FOREIGN KEY (fkey_labelid) REFERENCES label(id) ON DELETE CASCADE;
ALTER TABLE "scriptlabel" ADD CONSTRAINT "scriptlabel_fkey_scriptid_fkey" FOREIGN KEY (fkey_scriptid) REFERENCES script(id) ON DELETE CASCADE;
ALTER TABLE "sshtasks" ADD CONSTRAINT "sshtasks_sessionid_fkey" FOREIGN KEY (sessionid) REFERENCES sshsessions(sessionid) ON DELETE CASCADE;
ALTER TABLE "winrmtasks" ADD CONSTRAINT "winrmtasks_sessionid_fkey" FOREIGN KEY (sessionid) REFERENCES winrmsessions(sessionid) ON DELETE CASCADE;

CREATE INDEX username_idx ON public.account USING btree (username);
CREATE INDEX name_idx ON public.aifunctiondefinitions USING btree (name);
CREATE UNIQUE INDEX ix_doc_chunks_path_idx ON public.documentationchunks USING btree (path, chunkindex);
CREATE INDEX taskid_idx ON public.implanttasks USING btree (taskid);
CREATE UNIQUE INDEX netusers_host_username_domain_idx ON public.netusers USING btree (hostid, username, COALESCE(domain, ''::text));
CREATE INDEX scriptid_idx ON public.script USING btree (scriptid);
CREATE INDEX machineid_idx ON public.session USING btree (machineid);
CREATE INDEX sessionid_idx ON public.session USING btree (sessionid);
CREATE INDEX sshtasksessionid_idx ON public.sshtasks USING btree (sessionid);
CREATE INDEX winrmtasksessionid_idx ON public.winrmtasks USING btree (sessionid);
