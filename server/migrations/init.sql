--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;

--
-- Name: activity_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.activity_log (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    issue_id uuid,
    actor_type text,
    actor_id uuid,
    action text NOT NULL,
    details jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT activity_log_actor_type_check CHECK ((actor_type = ANY (ARRAY['member'::text, 'agent'::text, 'system'::text])))
);


--
-- Name: agent; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    name text NOT NULL,
    avatar_url text,
    runtime_mode text NOT NULL,
    runtime_config jsonb DEFAULT '{}'::jsonb NOT NULL,
    visibility text DEFAULT 'private'::text NOT NULL,
    status text DEFAULT 'offline'::text NOT NULL,
    max_concurrent_tasks integer DEFAULT 6 NOT NULL,
    owner_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    runtime_id uuid NOT NULL,
    instructions text DEFAULT ''::text NOT NULL,
    archived_at timestamp with time zone,
    archived_by uuid,
    custom_env jsonb DEFAULT '{}'::jsonb NOT NULL,
    custom_args jsonb DEFAULT '[]'::jsonb NOT NULL,
    mcp_config jsonb,
    model text,
    CONSTRAINT agent_description_length CHECK ((char_length(description) <= 255)),
    CONSTRAINT agent_runtime_mode_check CHECK ((runtime_mode = ANY (ARRAY['local'::text, 'cloud'::text]))),
    CONSTRAINT agent_status_check CHECK ((status = ANY (ARRAY['idle'::text, 'working'::text, 'blocked'::text, 'error'::text, 'offline'::text]))),
    CONSTRAINT agent_visibility_check CHECK ((visibility = ANY (ARRAY['workspace'::text, 'private'::text])))
);


--
-- Name: agent_runtime; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_runtime (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    daemon_id text,
    name text NOT NULL,
    runtime_mode text NOT NULL,
    provider text NOT NULL,
    status text DEFAULT 'offline'::text NOT NULL,
    device_info text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    last_seen_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_id uuid,
    legacy_daemon_id text,
    CONSTRAINT agent_runtime_runtime_mode_check CHECK ((runtime_mode = ANY (ARRAY['local'::text, 'cloud'::text]))),
    CONSTRAINT agent_runtime_status_check CHECK ((status = ANY (ARRAY['online'::text, 'offline'::text])))
);


--
-- Name: agent_skill; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_skill (
    agent_id uuid NOT NULL,
    skill_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: agent_task_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_task_queue (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    agent_id uuid NOT NULL,
    issue_id uuid,
    status text DEFAULT 'queued'::text NOT NULL,
    priority integer DEFAULT 0 NOT NULL,
    dispatched_at timestamp with time zone,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    result jsonb,
    error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    context jsonb,
    runtime_id uuid NOT NULL,
    session_id text,
    work_dir text,
    trigger_comment_id uuid,
    chat_session_id uuid,
    autopilot_run_id uuid,
    attempt integer DEFAULT 1 NOT NULL,
    max_attempts integer DEFAULT 2 NOT NULL,
    parent_task_id uuid,
    failure_reason text,
    last_heartbeat_at timestamp with time zone,
    trigger_summary text,
    force_fresh_session boolean DEFAULT false NOT NULL,
    CONSTRAINT agent_task_queue_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'dispatched'::text, 'running'::text, 'completed'::text, 'failed'::text, 'cancelled'::text])))
);


--
-- Name: attachment; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.attachment (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    issue_id uuid,
    comment_id uuid,
    uploader_type text NOT NULL,
    uploader_id uuid NOT NULL,
    filename text NOT NULL,
    url text NOT NULL,
    content_type text NOT NULL,
    size_bytes bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT attachment_uploader_type_check CHECK ((uploader_type = ANY (ARRAY['member'::text, 'agent'::text])))
);


--
-- Name: autopilot; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.autopilot (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    title text NOT NULL,
    description text,
    assignee_id uuid NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    execution_mode text DEFAULT 'create_issue'::text NOT NULL,
    issue_title_template text,
    created_by_type text NOT NULL,
    created_by_id uuid NOT NULL,
    last_run_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT autopilot_created_by_type_check CHECK ((created_by_type = ANY (ARRAY['member'::text, 'agent'::text]))),
    CONSTRAINT autopilot_execution_mode_check CHECK ((execution_mode = ANY (ARRAY['create_issue'::text, 'run_only'::text]))),
    CONSTRAINT autopilot_status_check CHECK ((status = ANY (ARRAY['active'::text, 'paused'::text, 'archived'::text])))
);


--
-- Name: autopilot_run; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.autopilot_run (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    autopilot_id uuid NOT NULL,
    trigger_id uuid,
    source text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    issue_id uuid,
    task_id uuid,
    triggered_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    failure_reason text,
    trigger_payload jsonb,
    result jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT autopilot_run_source_check CHECK ((source = ANY (ARRAY['schedule'::text, 'manual'::text, 'webhook'::text, 'api'::text]))),
    CONSTRAINT autopilot_run_status_check CHECK ((status = ANY (ARRAY['issue_created'::text, 'running'::text, 'completed'::text, 'failed'::text])))
);


--
-- Name: autopilot_trigger; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.autopilot_trigger (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    autopilot_id uuid NOT NULL,
    kind text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    cron_expression text,
    timezone text DEFAULT 'UTC'::text,
    next_run_at timestamp with time zone,
    webhook_token text,
    label text,
    last_fired_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT autopilot_trigger_kind_check CHECK ((kind = ANY (ARRAY['schedule'::text, 'webhook'::text, 'api'::text])))
);


--
-- Name: chat_message; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_message (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    chat_session_id uuid NOT NULL,
    role text NOT NULL,
    content text NOT NULL,
    task_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    failure_reason text,
    elapsed_ms bigint,
    CONSTRAINT chat_message_role_check CHECK ((role = ANY (ARRAY['user'::text, 'assistant'::text])))
);


--
-- Name: chat_session; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.chat_session (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    creator_id uuid NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    session_id text,
    work_dir text,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    unread_since timestamp with time zone,
    CONSTRAINT chat_session_status_check CHECK ((status = ANY (ARRAY['active'::text, 'archived'::text])))
);


--
-- Name: comment; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comment (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    issue_id uuid NOT NULL,
    author_type text NOT NULL,
    author_id uuid NOT NULL,
    content text NOT NULL,
    type text DEFAULT 'comment'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    parent_id uuid,
    workspace_id uuid NOT NULL,
    CONSTRAINT comment_author_type_check CHECK ((author_type = ANY (ARRAY['member'::text, 'agent'::text]))),
    CONSTRAINT comment_type_check CHECK ((type = ANY (ARRAY['comment'::text, 'status_change'::text, 'progress_update'::text, 'system'::text])))
);


--
-- Name: comment_reaction; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.comment_reaction (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    comment_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    actor_type text NOT NULL,
    actor_id uuid NOT NULL,
    emoji text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT comment_reaction_actor_type_check CHECK ((actor_type = ANY (ARRAY['member'::text, 'agent'::text])))
);


--
-- Name: daemon_connection; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.daemon_connection (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    agent_id uuid NOT NULL,
    daemon_id text NOT NULL,
    status text DEFAULT 'disconnected'::text NOT NULL,
    last_heartbeat_at timestamp with time zone,
    runtime_info jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT daemon_connection_status_check CHECK ((status = ANY (ARRAY['connected'::text, 'disconnected'::text])))
);


--
-- Name: daemon_token; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.daemon_token (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    token_hash text NOT NULL,
    workspace_id uuid NOT NULL,
    daemon_id text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: feedback; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.feedback (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    workspace_id uuid,
    message text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: inbox_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.inbox_item (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    recipient_type text NOT NULL,
    recipient_id uuid NOT NULL,
    type text NOT NULL,
    severity text DEFAULT 'info'::text NOT NULL,
    issue_id uuid,
    title text NOT NULL,
    body text,
    read boolean DEFAULT false NOT NULL,
    archived boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    actor_type text,
    actor_id uuid,
    details jsonb DEFAULT '{}'::jsonb,
    CONSTRAINT inbox_item_recipient_type_check CHECK ((recipient_type = ANY (ARRAY['member'::text, 'agent'::text]))),
    CONSTRAINT inbox_item_severity_check CHECK ((severity = ANY (ARRAY['action_required'::text, 'attention'::text, 'info'::text])))
);


--
-- Name: issue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.issue (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    title text NOT NULL,
    description text,
    status text DEFAULT 'backlog'::text NOT NULL,
    priority text DEFAULT 'none'::text NOT NULL,
    assignee_type text,
    assignee_id uuid,
    creator_type text NOT NULL,
    creator_id uuid NOT NULL,
    parent_issue_id uuid,
    acceptance_criteria jsonb DEFAULT '[]'::jsonb NOT NULL,
    context_refs jsonb DEFAULT '[]'::jsonb NOT NULL,
    "position" double precision DEFAULT 0 NOT NULL,
    due_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    number integer DEFAULT 0 NOT NULL,
    project_id uuid,
    origin_type text,
    origin_id uuid,
    first_executed_at timestamp with time zone,
    CONSTRAINT issue_assignee_type_check CHECK ((assignee_type = ANY (ARRAY['member'::text, 'agent'::text]))),
    CONSTRAINT issue_creator_type_check CHECK ((creator_type = ANY (ARRAY['member'::text, 'agent'::text]))),
    CONSTRAINT issue_origin_type_check CHECK ((origin_type = ANY (ARRAY['autopilot'::text, 'quick_create'::text]))),
    CONSTRAINT issue_priority_check CHECK ((priority = ANY (ARRAY['urgent'::text, 'high'::text, 'medium'::text, 'low'::text, 'none'::text]))),
    CONSTRAINT issue_status_check CHECK ((status = ANY (ARRAY['backlog'::text, 'todo'::text, 'in_progress'::text, 'in_review'::text, 'done'::text, 'blocked'::text, 'cancelled'::text])))
);


--
-- Name: issue_dependency; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.issue_dependency (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    issue_id uuid NOT NULL,
    depends_on_issue_id uuid NOT NULL,
    type text NOT NULL,
    CONSTRAINT issue_dependency_type_check CHECK ((type = ANY (ARRAY['blocks'::text, 'blocked_by'::text, 'related'::text])))
);


--
-- Name: issue_label; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.issue_label (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    name text NOT NULL,
    color text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: issue_reaction; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.issue_reaction (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    issue_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    actor_type text NOT NULL,
    actor_id uuid NOT NULL,
    emoji text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT issue_reaction_actor_type_check CHECK ((actor_type = ANY (ARRAY['member'::text, 'agent'::text])))
);


--
-- Name: issue_subscriber; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.issue_subscriber (
    issue_id uuid NOT NULL,
    user_type text NOT NULL,
    user_id uuid NOT NULL,
    reason text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT issue_subscriber_reason_check CHECK ((reason = ANY (ARRAY['creator'::text, 'assignee'::text, 'commenter'::text, 'mentioned'::text, 'manual'::text]))),
    CONSTRAINT issue_subscriber_user_type_check CHECK ((user_type = ANY (ARRAY['member'::text, 'agent'::text])))
);


--
-- Name: issue_to_label; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.issue_to_label (
    issue_id uuid NOT NULL,
    label_id uuid NOT NULL
);


--
-- Name: member; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.member (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT member_role_check CHECK ((role = ANY (ARRAY['owner'::text, 'admin'::text, 'member'::text])))
);


--
-- Name: notification_preference; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_preference (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    user_id uuid NOT NULL,
    preferences jsonb DEFAULT '{}'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: personal_access_token; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.personal_access_token (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    name text NOT NULL,
    token_hash text NOT NULL,
    token_prefix text NOT NULL,
    expires_at timestamp with time zone,
    last_used_at timestamp with time zone,
    revoked boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: pinned_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pinned_item (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    user_id uuid NOT NULL,
    item_type text NOT NULL,
    item_id uuid NOT NULL,
    "position" double precision DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT pinned_item_item_type_check CHECK ((item_type = ANY (ARRAY['issue'::text, 'project'::text])))
);


--
-- Name: project; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.project (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    title text NOT NULL,
    description text,
    icon text,
    status text DEFAULT 'planned'::text NOT NULL,
    lead_type text,
    lead_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    priority text DEFAULT 'none'::text NOT NULL,
    CONSTRAINT project_lead_type_check CHECK ((lead_type = ANY (ARRAY['member'::text, 'agent'::text]))),
    CONSTRAINT project_priority_check CHECK ((priority = ANY (ARRAY['urgent'::text, 'high'::text, 'medium'::text, 'low'::text, 'none'::text]))),
    CONSTRAINT project_status_check CHECK ((status = ANY (ARRAY['planned'::text, 'in_progress'::text, 'paused'::text, 'completed'::text, 'cancelled'::text])))
);


--
-- Name: project_resource; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.project_resource (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    project_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    resource_type text NOT NULL,
    resource_ref jsonb NOT NULL,
    label text,
    "position" integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid
);


--
-- Name: skill; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skill (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: skill_file; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.skill_file (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    skill_id uuid NOT NULL,
    path text NOT NULL,
    content text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: task_message; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.task_message (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    task_id uuid NOT NULL,
    seq integer NOT NULL,
    type text NOT NULL,
    tool text,
    content text,
    input jsonb,
    output text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: task_usage; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.task_usage (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    task_id uuid NOT NULL,
    provider text DEFAULT ''::text NOT NULL,
    model text NOT NULL,
    input_tokens bigint DEFAULT 0 NOT NULL,
    output_tokens bigint DEFAULT 0 NOT NULL,
    cache_read_tokens bigint DEFAULT 0 NOT NULL,
    cache_write_tokens bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."user" (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    avatar_url text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    onboarded_at timestamp with time zone,
    onboarding_questionnaire jsonb DEFAULT '{}'::jsonb NOT NULL,
    cloud_waitlist_email character varying(254),
    cloud_waitlist_reason text,
    starter_content_state text
);


--
-- Name: verification_code; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.verification_code (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email text NOT NULL,
    code text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    used boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    attempts integer DEFAULT 0 NOT NULL
);


--
-- Name: workspace; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.workspace (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    slug text NOT NULL,
    description text,
    settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    context text,
    repos jsonb DEFAULT '[]'::jsonb NOT NULL,
    issue_prefix text DEFAULT ''::text NOT NULL,
    issue_counter integer DEFAULT 0 NOT NULL
);


--
-- Name: workspace_invitation; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.workspace_invitation (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workspace_id uuid NOT NULL,
    inviter_id uuid NOT NULL,
    invitee_email text NOT NULL,
    invitee_user_id uuid,
    role text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone DEFAULT (now() + '7 days'::interval) NOT NULL,
    CONSTRAINT workspace_invitation_role_check CHECK ((role = ANY (ARRAY['admin'::text, 'member'::text]))),
    CONSTRAINT workspace_invitation_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'accepted'::text, 'declined'::text, 'expired'::text])))
);


--
-- Name: activity_log activity_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity_log
    ADD CONSTRAINT activity_log_pkey PRIMARY KEY (id);


--
-- Name: agent agent_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent
    ADD CONSTRAINT agent_pkey PRIMARY KEY (id);


--
-- Name: agent_runtime agent_runtime_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_runtime
    ADD CONSTRAINT agent_runtime_pkey PRIMARY KEY (id);


--
-- Name: agent_runtime agent_runtime_workspace_id_daemon_id_provider_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_runtime
    ADD CONSTRAINT agent_runtime_workspace_id_daemon_id_provider_key UNIQUE (workspace_id, daemon_id, provider);


--
-- Name: agent_skill agent_skill_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_skill
    ADD CONSTRAINT agent_skill_pkey PRIMARY KEY (agent_id, skill_id);


--
-- Name: agent_task_queue agent_task_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_task_queue
    ADD CONSTRAINT agent_task_queue_pkey PRIMARY KEY (id);


--
-- Name: agent agent_workspace_name_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent
    ADD CONSTRAINT agent_workspace_name_unique UNIQUE (workspace_id, name);


--
-- Name: attachment attachment_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attachment
    ADD CONSTRAINT attachment_pkey PRIMARY KEY (id);


--
-- Name: autopilot autopilot_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot
    ADD CONSTRAINT autopilot_pkey PRIMARY KEY (id);


--
-- Name: autopilot_run autopilot_run_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot_run
    ADD CONSTRAINT autopilot_run_pkey PRIMARY KEY (id);


--
-- Name: autopilot_trigger autopilot_trigger_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot_trigger
    ADD CONSTRAINT autopilot_trigger_pkey PRIMARY KEY (id);


--
-- Name: chat_message chat_message_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_pkey PRIMARY KEY (id);


--
-- Name: chat_session chat_session_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_session
    ADD CONSTRAINT chat_session_pkey PRIMARY KEY (id);


--
-- Name: comment comment_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment
    ADD CONSTRAINT comment_pkey PRIMARY KEY (id);


--
-- Name: comment_reaction comment_reaction_comment_id_actor_type_actor_id_emoji_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_reaction
    ADD CONSTRAINT comment_reaction_comment_id_actor_type_actor_id_emoji_key UNIQUE (comment_id, actor_type, actor_id, emoji);


--
-- Name: comment_reaction comment_reaction_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_reaction
    ADD CONSTRAINT comment_reaction_pkey PRIMARY KEY (id);


--
-- Name: daemon_connection daemon_connection_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daemon_connection
    ADD CONSTRAINT daemon_connection_pkey PRIMARY KEY (id);


--
-- Name: daemon_token daemon_token_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daemon_token
    ADD CONSTRAINT daemon_token_pkey PRIMARY KEY (id);


--
-- Name: feedback feedback_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.feedback
    ADD CONSTRAINT feedback_pkey PRIMARY KEY (id);


--
-- Name: inbox_item inbox_item_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inbox_item
    ADD CONSTRAINT inbox_item_pkey PRIMARY KEY (id);


--
-- Name: issue_dependency issue_dependency_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_dependency
    ADD CONSTRAINT issue_dependency_pkey PRIMARY KEY (id);


--
-- Name: issue_label issue_label_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_label
    ADD CONSTRAINT issue_label_pkey PRIMARY KEY (id);


--
-- Name: issue issue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue
    ADD CONSTRAINT issue_pkey PRIMARY KEY (id);


--
-- Name: issue_reaction issue_reaction_issue_id_actor_type_actor_id_emoji_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reaction
    ADD CONSTRAINT issue_reaction_issue_id_actor_type_actor_id_emoji_key UNIQUE (issue_id, actor_type, actor_id, emoji);


--
-- Name: issue_reaction issue_reaction_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reaction
    ADD CONSTRAINT issue_reaction_pkey PRIMARY KEY (id);


--
-- Name: issue_subscriber issue_subscriber_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_subscriber
    ADD CONSTRAINT issue_subscriber_pkey PRIMARY KEY (issue_id, user_type, user_id);


--
-- Name: issue_to_label issue_to_label_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_to_label
    ADD CONSTRAINT issue_to_label_pkey PRIMARY KEY (issue_id, label_id);


--
-- Name: member member_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member
    ADD CONSTRAINT member_pkey PRIMARY KEY (id);


--
-- Name: member member_workspace_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member
    ADD CONSTRAINT member_workspace_id_user_id_key UNIQUE (workspace_id, user_id);


--
-- Name: notification_preference notification_preference_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_preference
    ADD CONSTRAINT notification_preference_pkey PRIMARY KEY (id);


--
-- Name: notification_preference notification_preference_workspace_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_preference
    ADD CONSTRAINT notification_preference_workspace_id_user_id_key UNIQUE (workspace_id, user_id);


--
-- Name: personal_access_token personal_access_token_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_token
    ADD CONSTRAINT personal_access_token_pkey PRIMARY KEY (id);


--
-- Name: pinned_item pinned_item_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pinned_item
    ADD CONSTRAINT pinned_item_pkey PRIMARY KEY (id);


--
-- Name: pinned_item pinned_item_workspace_id_user_id_item_type_item_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pinned_item
    ADD CONSTRAINT pinned_item_workspace_id_user_id_item_type_item_id_key UNIQUE (workspace_id, user_id, item_type, item_id);


--
-- Name: project project_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project
    ADD CONSTRAINT project_pkey PRIMARY KEY (id);


--
-- Name: project_resource project_resource_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_resource
    ADD CONSTRAINT project_resource_pkey PRIMARY KEY (id);


--
-- Name: project_resource project_resource_project_id_resource_type_resource_ref_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_resource
    ADD CONSTRAINT project_resource_project_id_resource_type_resource_ref_key UNIQUE (project_id, resource_type, resource_ref);


--
-- Name: skill_file skill_file_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skill_file
    ADD CONSTRAINT skill_file_pkey PRIMARY KEY (id);


--
-- Name: skill_file skill_file_skill_id_path_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skill_file
    ADD CONSTRAINT skill_file_skill_id_path_key UNIQUE (skill_id, path);


--
-- Name: skill skill_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skill
    ADD CONSTRAINT skill_pkey PRIMARY KEY (id);


--
-- Name: skill skill_workspace_id_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skill
    ADD CONSTRAINT skill_workspace_id_name_key UNIQUE (workspace_id, name);


--
-- Name: task_message task_message_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.task_message
    ADD CONSTRAINT task_message_pkey PRIMARY KEY (id);


--
-- Name: task_usage task_usage_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.task_usage
    ADD CONSTRAINT task_usage_pkey PRIMARY KEY (id);


--
-- Name: task_usage task_usage_task_id_provider_model_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.task_usage
    ADD CONSTRAINT task_usage_task_id_provider_model_key UNIQUE (task_id, provider, model);


--
-- Name: daemon_connection uq_daemon_agent; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daemon_connection
    ADD CONSTRAINT uq_daemon_agent UNIQUE (agent_id, daemon_id);


--
-- Name: issue uq_issue_workspace_number; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue
    ADD CONSTRAINT uq_issue_workspace_number UNIQUE (workspace_id, number);


--
-- Name: user user_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT user_email_key UNIQUE (email);


--
-- Name: user user_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT user_pkey PRIMARY KEY (id);


--
-- Name: verification_code verification_code_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.verification_code
    ADD CONSTRAINT verification_code_pkey PRIMARY KEY (id);


--
-- Name: workspace_invitation workspace_invitation_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workspace_invitation
    ADD CONSTRAINT workspace_invitation_pkey PRIMARY KEY (id);


--
-- Name: workspace workspace_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workspace
    ADD CONSTRAINT workspace_pkey PRIMARY KEY (id);


--
-- Name: workspace workspace_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workspace
    ADD CONSTRAINT workspace_slug_key UNIQUE (slug);


--
-- Name: idx_activity_log_issue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_activity_log_issue ON public.activity_log USING btree (issue_id);


--
-- Name: idx_agent_runtime_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_runtime_status ON public.agent_runtime USING btree (workspace_id, status);


--
-- Name: idx_agent_runtime_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_runtime_workspace ON public.agent_runtime USING btree (workspace_id);


--
-- Name: idx_agent_skill_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_skill_agent ON public.agent_skill USING btree (agent_id);


--
-- Name: idx_agent_skill_skill; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_skill_skill ON public.agent_skill USING btree (skill_id);


--
-- Name: idx_agent_task_queue_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_task_queue_agent ON public.agent_task_queue USING btree (agent_id, status);


--
-- Name: idx_agent_task_queue_chat_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_task_queue_chat_pending ON public.agent_task_queue USING btree (chat_session_id, created_at DESC) WHERE ((chat_session_id IS NOT NULL) AND (status = ANY (ARRAY['queued'::text, 'dispatched'::text, 'running'::text])));


--
-- Name: idx_agent_task_queue_issue_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_task_queue_issue_id ON public.agent_task_queue USING btree (issue_id);


--
-- Name: idx_agent_task_queue_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_task_queue_parent ON public.agent_task_queue USING btree (parent_task_id);


--
-- Name: idx_agent_task_queue_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_task_queue_pending ON public.agent_task_queue USING btree (agent_id, priority DESC, created_at) WHERE (status = ANY (ARRAY['queued'::text, 'dispatched'::text]));


--
-- Name: idx_agent_task_queue_runtime_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_task_queue_runtime_pending ON public.agent_task_queue USING btree (runtime_id, priority DESC, created_at) WHERE (status = ANY (ARRAY['queued'::text, 'dispatched'::text]));


--
-- Name: idx_agent_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_workspace ON public.agent USING btree (workspace_id);


--
-- Name: idx_attachment_comment; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_attachment_comment ON public.attachment USING btree (comment_id) WHERE (comment_id IS NOT NULL);


--
-- Name: idx_attachment_issue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_attachment_issue ON public.attachment USING btree (issue_id) WHERE (issue_id IS NOT NULL);


--
-- Name: idx_attachment_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_attachment_workspace ON public.attachment USING btree (workspace_id);


--
-- Name: idx_autopilot_assignee; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_autopilot_assignee ON public.autopilot USING btree (assignee_id);


--
-- Name: idx_autopilot_run_autopilot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_autopilot_run_autopilot ON public.autopilot_run USING btree (autopilot_id, created_at DESC);


--
-- Name: idx_autopilot_run_issue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_autopilot_run_issue ON public.autopilot_run USING btree (issue_id) WHERE (issue_id IS NOT NULL);


--
-- Name: idx_autopilot_run_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_autopilot_run_status ON public.autopilot_run USING btree (autopilot_id, status) WHERE (status = ANY (ARRAY['issue_created'::text, 'running'::text]));


--
-- Name: idx_autopilot_trigger_autopilot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_autopilot_trigger_autopilot ON public.autopilot_trigger USING btree (autopilot_id);


--
-- Name: idx_autopilot_trigger_next_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_autopilot_trigger_next_run ON public.autopilot_trigger USING btree (next_run_at) WHERE ((enabled = true) AND (kind = 'schedule'::text));


--
-- Name: idx_autopilot_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_autopilot_workspace ON public.autopilot USING btree (workspace_id);


--
-- Name: idx_chat_message_session; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_message_session ON public.chat_message USING btree (chat_session_id, created_at);


--
-- Name: idx_chat_session_creator; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_session_creator ON public.chat_session USING btree (creator_id, workspace_id);


--
-- Name: idx_chat_session_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chat_session_workspace ON public.chat_session USING btree (workspace_id);


--
-- Name: idx_comment_issue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comment_issue ON public.comment USING btree (issue_id);


--
-- Name: idx_comment_reaction_comment_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comment_reaction_comment_id ON public.comment_reaction USING btree (comment_id);


--
-- Name: idx_daemon_token_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_daemon_token_hash ON public.daemon_token USING btree (token_hash);


--
-- Name: idx_daemon_token_workspace_daemon; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_daemon_token_workspace_daemon ON public.daemon_token USING btree (workspace_id, daemon_id);


--
-- Name: idx_feedback_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_feedback_user_created ON public.feedback USING btree (user_id, created_at DESC);


--
-- Name: idx_inbox_recipient; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_inbox_recipient ON public.inbox_item USING btree (recipient_type, recipient_id, read);


--
-- Name: idx_invitation_invitee_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_invitation_invitee_email ON public.workspace_invitation USING btree (invitee_email) WHERE (status = 'pending'::text);


--
-- Name: idx_invitation_invitee_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_invitation_invitee_user ON public.workspace_invitation USING btree (invitee_user_id) WHERE (status = 'pending'::text);


--
-- Name: idx_invitation_unique_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_invitation_unique_pending ON public.workspace_invitation USING btree (workspace_id, invitee_email) WHERE (status = 'pending'::text);


--
-- Name: idx_issue_assignee; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_assignee ON public.issue USING btree (assignee_type, assignee_id);


--
-- Name: idx_issue_first_executed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_first_executed_at ON public.issue USING btree (workspace_id, first_executed_at) WHERE (first_executed_at IS NOT NULL);


--
-- Name: idx_issue_origin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_origin ON public.issue USING btree (origin_type, origin_id) WHERE (origin_type IS NOT NULL);


--
-- Name: idx_issue_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_parent ON public.issue USING btree (parent_issue_id);


--
-- Name: idx_issue_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_project ON public.issue USING btree (project_id);


--
-- Name: idx_issue_reaction_issue_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_reaction_issue_id ON public.issue_reaction USING btree (issue_id);


--
-- Name: idx_issue_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_status ON public.issue USING btree (workspace_id, status);


--
-- Name: idx_issue_subscriber_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_subscriber_user ON public.issue_subscriber USING btree (user_type, user_id);


--
-- Name: idx_issue_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_workspace ON public.issue USING btree (workspace_id);


--
-- Name: idx_issue_workspace_number; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_workspace_number ON public.issue USING btree (workspace_id, number);


--
-- Name: idx_member_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_member_workspace ON public.member USING btree (workspace_id);


--
-- Name: idx_one_pending_task_per_issue_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_one_pending_task_per_issue_agent ON public.agent_task_queue USING btree (issue_id, agent_id) WHERE (status = ANY (ARRAY['queued'::text, 'dispatched'::text]));


--
-- Name: idx_pat_token_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_pat_token_hash ON public.personal_access_token USING btree (token_hash);


--
-- Name: idx_pat_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_pat_user ON public.personal_access_token USING btree (user_id, revoked);


--
-- Name: idx_pinned_item_user_ws; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_pinned_item_user_ws ON public.pinned_item USING btree (workspace_id, user_id, "position");


--
-- Name: idx_project_resource_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_resource_project ON public.project_resource USING btree (project_id, "position");


--
-- Name: idx_project_resource_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_resource_workspace ON public.project_resource USING btree (workspace_id);


--
-- Name: idx_project_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_workspace ON public.project USING btree (workspace_id);


--
-- Name: idx_skill_file_skill; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_skill_file_skill ON public.skill_file USING btree (skill_id);


--
-- Name: idx_skill_workspace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_skill_workspace ON public.skill USING btree (workspace_id);


--
-- Name: idx_task_message_task_id_seq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_task_message_task_id_seq ON public.task_message USING btree (task_id, seq);


--
-- Name: idx_task_usage_task_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_task_usage_task_id ON public.task_usage USING btree (task_id);


--
-- Name: idx_verification_code_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_verification_code_email ON public.verification_code USING btree (email, used, expires_at);


--
-- Name: issue_label_workspace_name_lower_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX issue_label_workspace_name_lower_idx ON public.issue_label USING btree (workspace_id, lower(name));


--
-- Name: activity_log activity_log_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity_log
    ADD CONSTRAINT activity_log_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: activity_log activity_log_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity_log
    ADD CONSTRAINT activity_log_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: agent agent_archived_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent
    ADD CONSTRAINT agent_archived_by_fkey FOREIGN KEY (archived_by) REFERENCES public."user"(id);


--
-- Name: agent agent_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent
    ADD CONSTRAINT agent_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public."user"(id);


--
-- Name: agent agent_runtime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent
    ADD CONSTRAINT agent_runtime_id_fkey FOREIGN KEY (runtime_id) REFERENCES public.agent_runtime(id) ON DELETE RESTRICT;


--
-- Name: agent_runtime agent_runtime_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_runtime
    ADD CONSTRAINT agent_runtime_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public."user"(id);


--
-- Name: agent_runtime agent_runtime_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_runtime
    ADD CONSTRAINT agent_runtime_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: agent_skill agent_skill_agent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_skill
    ADD CONSTRAINT agent_skill_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES public.agent(id) ON DELETE CASCADE;


--
-- Name: agent_skill agent_skill_skill_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_skill
    ADD CONSTRAINT agent_skill_skill_id_fkey FOREIGN KEY (skill_id) REFERENCES public.skill(id) ON DELETE CASCADE;


--
-- Name: agent_task_queue agent_task_queue_agent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_task_queue
    ADD CONSTRAINT agent_task_queue_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES public.agent(id) ON DELETE CASCADE;


--
-- Name: agent_task_queue agent_task_queue_autopilot_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_task_queue
    ADD CONSTRAINT agent_task_queue_autopilot_run_id_fkey FOREIGN KEY (autopilot_run_id) REFERENCES public.autopilot_run(id) ON DELETE SET NULL;


--
-- Name: agent_task_queue agent_task_queue_chat_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_task_queue
    ADD CONSTRAINT agent_task_queue_chat_session_id_fkey FOREIGN KEY (chat_session_id) REFERENCES public.chat_session(id) ON DELETE SET NULL;


--
-- Name: agent_task_queue agent_task_queue_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_task_queue
    ADD CONSTRAINT agent_task_queue_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: agent_task_queue agent_task_queue_parent_task_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_task_queue
    ADD CONSTRAINT agent_task_queue_parent_task_id_fkey FOREIGN KEY (parent_task_id) REFERENCES public.agent_task_queue(id) ON DELETE SET NULL;


--
-- Name: agent_task_queue agent_task_queue_runtime_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_task_queue
    ADD CONSTRAINT agent_task_queue_runtime_id_fkey FOREIGN KEY (runtime_id) REFERENCES public.agent_runtime(id) ON DELETE CASCADE;


--
-- Name: agent_task_queue agent_task_queue_trigger_comment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_task_queue
    ADD CONSTRAINT agent_task_queue_trigger_comment_id_fkey FOREIGN KEY (trigger_comment_id) REFERENCES public.comment(id) ON DELETE SET NULL;


--
-- Name: agent agent_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent
    ADD CONSTRAINT agent_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: attachment attachment_comment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attachment
    ADD CONSTRAINT attachment_comment_id_fkey FOREIGN KEY (comment_id) REFERENCES public.comment(id) ON DELETE CASCADE;


--
-- Name: attachment attachment_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attachment
    ADD CONSTRAINT attachment_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: attachment attachment_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.attachment
    ADD CONSTRAINT attachment_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: autopilot autopilot_assignee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot
    ADD CONSTRAINT autopilot_assignee_id_fkey FOREIGN KEY (assignee_id) REFERENCES public.agent(id) ON DELETE CASCADE;


--
-- Name: autopilot_run autopilot_run_autopilot_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot_run
    ADD CONSTRAINT autopilot_run_autopilot_id_fkey FOREIGN KEY (autopilot_id) REFERENCES public.autopilot(id) ON DELETE CASCADE;


--
-- Name: autopilot_run autopilot_run_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot_run
    ADD CONSTRAINT autopilot_run_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE SET NULL;


--
-- Name: autopilot_run autopilot_run_task_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot_run
    ADD CONSTRAINT autopilot_run_task_id_fkey FOREIGN KEY (task_id) REFERENCES public.agent_task_queue(id) ON DELETE SET NULL;


--
-- Name: autopilot_run autopilot_run_trigger_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot_run
    ADD CONSTRAINT autopilot_run_trigger_id_fkey FOREIGN KEY (trigger_id) REFERENCES public.autopilot_trigger(id) ON DELETE SET NULL;


--
-- Name: autopilot_trigger autopilot_trigger_autopilot_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot_trigger
    ADD CONSTRAINT autopilot_trigger_autopilot_id_fkey FOREIGN KEY (autopilot_id) REFERENCES public.autopilot(id) ON DELETE CASCADE;


--
-- Name: autopilot autopilot_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.autopilot
    ADD CONSTRAINT autopilot_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: chat_message chat_message_chat_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_message
    ADD CONSTRAINT chat_message_chat_session_id_fkey FOREIGN KEY (chat_session_id) REFERENCES public.chat_session(id) ON DELETE CASCADE;


--
-- Name: chat_session chat_session_agent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_session
    ADD CONSTRAINT chat_session_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES public.agent(id) ON DELETE CASCADE;


--
-- Name: chat_session chat_session_creator_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_session
    ADD CONSTRAINT chat_session_creator_id_fkey FOREIGN KEY (creator_id) REFERENCES public."user"(id) ON DELETE CASCADE;


--
-- Name: chat_session chat_session_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_session
    ADD CONSTRAINT chat_session_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: comment comment_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment
    ADD CONSTRAINT comment_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: comment comment_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment
    ADD CONSTRAINT comment_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.comment(id) ON DELETE CASCADE;


--
-- Name: comment_reaction comment_reaction_comment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_reaction
    ADD CONSTRAINT comment_reaction_comment_id_fkey FOREIGN KEY (comment_id) REFERENCES public.comment(id) ON DELETE CASCADE;


--
-- Name: comment_reaction comment_reaction_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment_reaction
    ADD CONSTRAINT comment_reaction_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: comment comment_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.comment
    ADD CONSTRAINT comment_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: daemon_connection daemon_connection_agent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daemon_connection
    ADD CONSTRAINT daemon_connection_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES public.agent(id) ON DELETE CASCADE;


--
-- Name: daemon_token daemon_token_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daemon_token
    ADD CONSTRAINT daemon_token_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: feedback feedback_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.feedback
    ADD CONSTRAINT feedback_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;


--
-- Name: feedback feedback_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.feedback
    ADD CONSTRAINT feedback_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE SET NULL;


--
-- Name: inbox_item inbox_item_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inbox_item
    ADD CONSTRAINT inbox_item_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: inbox_item inbox_item_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.inbox_item
    ADD CONSTRAINT inbox_item_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: issue_dependency issue_dependency_depends_on_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_dependency
    ADD CONSTRAINT issue_dependency_depends_on_issue_id_fkey FOREIGN KEY (depends_on_issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: issue_dependency issue_dependency_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_dependency
    ADD CONSTRAINT issue_dependency_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: issue_label issue_label_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_label
    ADD CONSTRAINT issue_label_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: issue issue_parent_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue
    ADD CONSTRAINT issue_parent_issue_id_fkey FOREIGN KEY (parent_issue_id) REFERENCES public.issue(id) ON DELETE SET NULL;


--
-- Name: issue issue_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue
    ADD CONSTRAINT issue_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.project(id) ON DELETE SET NULL;


--
-- Name: issue_reaction issue_reaction_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reaction
    ADD CONSTRAINT issue_reaction_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: issue_reaction issue_reaction_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reaction
    ADD CONSTRAINT issue_reaction_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: issue_subscriber issue_subscriber_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_subscriber
    ADD CONSTRAINT issue_subscriber_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: issue_to_label issue_to_label_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_to_label
    ADD CONSTRAINT issue_to_label_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.issue(id) ON DELETE CASCADE;


--
-- Name: issue_to_label issue_to_label_label_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_to_label
    ADD CONSTRAINT issue_to_label_label_id_fkey FOREIGN KEY (label_id) REFERENCES public.issue_label(id) ON DELETE CASCADE;


--
-- Name: issue issue_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue
    ADD CONSTRAINT issue_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: member member_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member
    ADD CONSTRAINT member_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;


--
-- Name: member member_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member
    ADD CONSTRAINT member_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: notification_preference notification_preference_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_preference
    ADD CONSTRAINT notification_preference_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;


--
-- Name: notification_preference notification_preference_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_preference
    ADD CONSTRAINT notification_preference_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: personal_access_token personal_access_token_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_token
    ADD CONSTRAINT personal_access_token_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;


--
-- Name: pinned_item pinned_item_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pinned_item
    ADD CONSTRAINT pinned_item_user_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;


--
-- Name: pinned_item pinned_item_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pinned_item
    ADD CONSTRAINT pinned_item_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: project_resource project_resource_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_resource
    ADD CONSTRAINT project_resource_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.project(id) ON DELETE CASCADE;


--
-- Name: project_resource project_resource_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project_resource
    ADD CONSTRAINT project_resource_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: project project_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.project
    ADD CONSTRAINT project_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: skill skill_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skill
    ADD CONSTRAINT skill_created_by_fkey FOREIGN KEY (created_by) REFERENCES public."user"(id);


--
-- Name: skill_file skill_file_skill_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skill_file
    ADD CONSTRAINT skill_file_skill_id_fkey FOREIGN KEY (skill_id) REFERENCES public.skill(id) ON DELETE CASCADE;


--
-- Name: skill skill_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.skill
    ADD CONSTRAINT skill_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- Name: task_message task_message_task_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.task_message
    ADD CONSTRAINT task_message_task_id_fkey FOREIGN KEY (task_id) REFERENCES public.agent_task_queue(id) ON DELETE CASCADE;


--
-- Name: task_usage task_usage_task_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.task_usage
    ADD CONSTRAINT task_usage_task_id_fkey FOREIGN KEY (task_id) REFERENCES public.agent_task_queue(id) ON DELETE CASCADE;


--
-- Name: workspace_invitation workspace_invitation_invitee_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workspace_invitation
    ADD CONSTRAINT workspace_invitation_invitee_user_id_fkey FOREIGN KEY (invitee_user_id) REFERENCES public."user"(id);


--
-- Name: workspace_invitation workspace_invitation_inviter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workspace_invitation
    ADD CONSTRAINT workspace_invitation_inviter_id_fkey FOREIGN KEY (inviter_id) REFERENCES public."user"(id);


--
-- Name: workspace_invitation workspace_invitation_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.workspace_invitation
    ADD CONSTRAINT workspace_invitation_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.workspace(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
