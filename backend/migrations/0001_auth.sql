-- +goose Up
--
-- PostgreSQL database dump
--


-- Dumped from database version 16.10 (Homebrew)
-- Dumped by pg_dump version 16.10 (Homebrew)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: ai_generated_question_answers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_generated_question_answers (
    question_version_id uuid NOT NULL,
    value jsonb NOT NULL,
    explanation text DEFAULT ''::text NOT NULL,
    prompt_version text NOT NULL,
    model text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ai_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_runs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    kind text NOT NULL,
    prompt_version text NOT NULL,
    model text DEFAULT ''::text NOT NULL,
    input_ref text DEFAULT ''::text NOT NULL,
    output jsonb,
    prompt_tokens integer DEFAULT 0 NOT NULL,
    completion_tokens integer DEFAULT 0 NOT NULL,
    duration_ms integer DEFAULT 0 NOT NULL,
    error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: answer_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.answer_keys (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    question_version_id uuid NOT NULL,
    value jsonb NOT NULL,
    authority text NOT NULL,
    explanation text DEFAULT ''::text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT answer_keys_authority_check CHECK ((authority = ANY (ARRAY['official'::text, 'human_verified'::text])))
);


--
-- Name: audit_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    actor_user_id uuid,
    action text NOT NULL,
    object_type text NOT NULL,
    object_id text DEFAULT ''::text NOT NULL,
    detail jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: auth_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.auth_sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: exam_levels; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.exam_levels (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    exam_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: exams; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.exams (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: grading_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.grading_results (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    item_id uuid NOT NULL,
    source text NOT NULL,
    status text NOT NULL,
    answer_authority text,
    correct_value jsonb,
    user_value jsonb,
    explanation text,
    explanation_source text,
    ai_run_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT grading_results_answer_authority_check CHECK ((answer_authority = ANY (ARRAY['official'::text, 'human_verified'::text]))),
    CONSTRAINT grading_results_explanation_source_check CHECK ((explanation_source = ANY (ARRAY['official'::text, 'human_verified'::text, 'ai'::text]))),
    CONSTRAINT grading_results_source_check CHECK ((source = ANY (ARRAY['deterministic'::text, 'ai'::text]))),
    CONSTRAINT grading_results_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'correct'::text, 'incorrect'::text, 'unanswered'::text, 'failed'::text])))
);


--
-- Name: import_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.import_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    import_job_id uuid NOT NULL,
    "position" integer NOT NULL,
    raw_excerpt text DEFAULT ''::text NOT NULL,
    ai_draft jsonb,
    anomalies jsonb DEFAULT '[]'::jsonb NOT NULL,
    review_status text DEFAULT 'pending'::text NOT NULL,
    published_question_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT import_items_review_status_check CHECK ((review_status = ANY (ARRAY['pending'::text, 'approved'::text, 'published'::text, 'rejected'::text])))
);


--
-- Name: import_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.import_jobs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    created_by uuid NOT NULL,
    file_name text NOT NULL,
    stored_path text DEFAULT ''::text NOT NULL,
    file_sha256 text DEFAULT ''::text NOT NULL,
    mime_type text DEFAULT ''::text NOT NULL,
    size_bytes bigint DEFAULT 0 NOT NULL,
    status text DEFAULT 'uploaded'::text NOT NULL,
    stage_error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    extracted_text text DEFAULT ''::text NOT NULL,
    stage_times jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT import_jobs_status_check CHECK ((status = ANY (ARRAY['uploaded'::text, 'extracting'::text, 'structuring'::text, 'review_ready'::text, 'published'::text, 'failed'::text])))
);


--
-- Name: issue_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.issue_reports (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    question_id uuid NOT NULL,
    question_version_id uuid NOT NULL,
    practice_item_id uuid,
    session_id uuid,
    target_type text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    context jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'open'::text NOT NULL,
    resolution_note text DEFAULT ''::text NOT NULL,
    handled_by uuid,
    handled_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT issue_reports_status_check CHECK ((status = ANY (ARRAY['open'::text, 'resolved'::text, 'dismissed'::text]))),
    CONSTRAINT issue_reports_target_type_check CHECK ((target_type = ANY (ARRAY['stem'::text, 'answer'::text, 'explanation'::text, 'classification'::text, 'ai_grading'::text])))
);


--
-- Name: jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    kind text NOT NULL,
    payload jsonb NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    max_attempts integer DEFAULT 3 NOT NULL,
    available_at timestamp with time zone DEFAULT now() NOT NULL,
    locked_by text,
    locked_until timestamp with time zone,
    last_error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT jobs_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text])))
);


--
-- Name: knowledge_points; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_points (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    exam_id uuid NOT NULL,
    level_id uuid NOT NULL,
    subject_id uuid NOT NULL,
    parent_id uuid,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    common_mistakes text DEFAULT ''::text NOT NULL,
    examples text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT knowledge_points_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'published'::text, 'retired'::text])))
);


--
-- Name: material_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.material_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    material_id uuid NOT NULL,
    version_no integer NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    content text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: materials; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.materials (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    current_version_id uuid,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: password_reset_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.password_reset_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: practice_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.practice_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    question_id uuid NOT NULL,
    question_version_id uuid NOT NULL,
    "position" integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: practice_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.practice_sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    level_id uuid NOT NULL,
    subject_id uuid,
    scope jsonb DEFAULT '{}'::jsonb NOT NULL,
    requested_count integer NOT NULL,
    submit_key text,
    submit_hash text,
    submitted_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    ai_summary text DEFAULT ''::text NOT NULL,
    ai_summary_status text DEFAULT 'not_requested'::text NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT practice_sessions_ai_summary_status_check CHECK ((ai_summary_status = ANY (ARRAY['not_requested'::text, 'pending'::text, 'completed'::text, 'failed'::text]))),
    CONSTRAINT practice_sessions_status_check CHECK ((status = ANY (ARRAY['generating'::text, 'active'::text, 'grading'::text, 'completed'::text, 'analysis_failed'::text, 'generation_failed'::text])))
);


--
-- Name: question_ai_explanations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.question_ai_explanations (
    question_version_id uuid NOT NULL,
    prompt_version text NOT NULL,
    model text DEFAULT ''::text NOT NULL,
    explanation text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: question_version_knowledge_points; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.question_version_knowledge_points (
    question_version_id uuid NOT NULL,
    knowledge_point_id uuid NOT NULL
);


--
-- Name: question_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.question_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    question_id uuid NOT NULL,
    version_no integer NOT NULL,
    type text NOT NULL,
    stem text NOT NULL,
    material_version_id uuid,
    options jsonb,
    level_id uuid NOT NULL,
    subject_id uuid NOT NULL,
    source_section_id uuid,
    difficulty integer DEFAULT 3 NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    source_order integer,
    CONSTRAINT question_versions_difficulty_check CHECK (((difficulty >= 1) AND (difficulty <= 5))),
    CONSTRAINT question_versions_type_check CHECK ((type = ANY (ARRAY['single_choice'::text, 'multiple_choice'::text, 'fill_blank'::text, 'short_answer'::text])))
);


--
-- Name: questions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.questions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    current_version_id uuid,
    published_version_id uuid,
    status text DEFAULT 'draft'::text NOT NULL,
    has_answer boolean DEFAULT false NOT NULL,
    created_by uuid,
    published_at timestamp with time zone,
    retired_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT questions_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'in_review'::text, 'published'::text, 'retired'::text])))
);


--
-- Name: rate_limit_counters; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rate_limit_counters (
    key text NOT NULL,
    window_start timestamp with time zone NOT NULL,
    count integer DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: source_sections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.source_sections (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    source_id uuid NOT NULL,
    name text NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: sources; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sources (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    kind text NOT NULL,
    author text DEFAULT ''::text NOT NULL,
    publisher text DEFAULT ''::text NOT NULL,
    year integer,
    license_note text DEFAULT ''::text NOT NULL,
    internal_note text DEFAULT ''::text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT sources_kind_check CHECK ((kind = ANY (ARRAY['book'::text, 'past_exam'::text, 'self_made'::text, 'ai_generated'::text])))
);


--
-- Name: subjects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.subjects (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    exam_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_answers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_answers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    item_id uuid NOT NULL,
    user_id uuid NOT NULL,
    value jsonb,
    marked_for_review boolean DEFAULT false NOT NULL,
    saved_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_knowledge_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_knowledge_stats (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    knowledge_point_id uuid NOT NULL,
    confirmed_answered integer DEFAULT 0 NOT NULL,
    confirmed_correct integer DEFAULT 0 NOT NULL,
    recent_answered integer DEFAULT 0 NOT NULL,
    recent_correct integer DEFAULT 0 NOT NULL,
    ai_answered integer DEFAULT 0 NOT NULL,
    ai_correct integer DEFAULT 0 NOT NULL,
    consecutive_wrong integer DEFAULT 0 NOT NULL,
    last_practiced_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_learning_memory; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_learning_memory (
    user_id uuid NOT NULL,
    reset_at timestamp with time zone,
    ai_advice text DEFAULT ''::text NOT NULL,
    ai_advice_status text DEFAULT 'not_requested'::text NOT NULL,
    ai_advice_updated_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_learning_memory_ai_advice_status_check CHECK ((ai_advice_status = ANY (ARRAY['not_requested'::text, 'pending'::text, 'completed'::text, 'failed'::text])))
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email text NOT NULL,
    email_normalized text NOT NULL,
    password_hash text NOT NULL,
    role text DEFAULT 'learner'::text NOT NULL,
    default_level_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT users_role_check CHECK ((role = ANY (ARRAY['learner'::text, 'admin'::text])))
);


--
-- Name: ai_generated_question_answers ai_generated_question_answers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_generated_question_answers
    ADD CONSTRAINT ai_generated_question_answers_pkey PRIMARY KEY (question_version_id);


--
-- Name: ai_runs ai_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_runs
    ADD CONSTRAINT ai_runs_pkey PRIMARY KEY (id);


--
-- Name: answer_keys answer_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.answer_keys
    ADD CONSTRAINT answer_keys_pkey PRIMARY KEY (id);


--
-- Name: answer_keys answer_keys_question_version_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.answer_keys
    ADD CONSTRAINT answer_keys_question_version_id_key UNIQUE (question_version_id);


--
-- Name: audit_logs audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);


--
-- Name: auth_sessions auth_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auth_sessions
    ADD CONSTRAINT auth_sessions_pkey PRIMARY KEY (id);


--
-- Name: auth_sessions auth_sessions_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auth_sessions
    ADD CONSTRAINT auth_sessions_token_hash_key UNIQUE (token_hash);


--
-- Name: exam_levels exam_levels_exam_id_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.exam_levels
    ADD CONSTRAINT exam_levels_exam_id_code_key UNIQUE (exam_id, code);


--
-- Name: exam_levels exam_levels_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.exam_levels
    ADD CONSTRAINT exam_levels_pkey PRIMARY KEY (id);


--
-- Name: exams exams_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.exams
    ADD CONSTRAINT exams_code_key UNIQUE (code);


--
-- Name: exams exams_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.exams
    ADD CONSTRAINT exams_pkey PRIMARY KEY (id);


--
-- Name: grading_results grading_results_item_id_source_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.grading_results
    ADD CONSTRAINT grading_results_item_id_source_key UNIQUE (item_id, source);


--
-- Name: grading_results grading_results_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.grading_results
    ADD CONSTRAINT grading_results_pkey PRIMARY KEY (id);


--
-- Name: import_items import_items_import_job_id_position_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_items
    ADD CONSTRAINT import_items_import_job_id_position_key UNIQUE (import_job_id, "position");


--
-- Name: import_items import_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_items
    ADD CONSTRAINT import_items_pkey PRIMARY KEY (id);


--
-- Name: import_jobs import_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_jobs
    ADD CONSTRAINT import_jobs_pkey PRIMARY KEY (id);


--
-- Name: issue_reports issue_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reports
    ADD CONSTRAINT issue_reports_pkey PRIMARY KEY (id);


--
-- Name: jobs jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);


--
-- Name: knowledge_points knowledge_points_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_points
    ADD CONSTRAINT knowledge_points_pkey PRIMARY KEY (id);


--
-- Name: material_versions material_versions_material_id_version_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_versions
    ADD CONSTRAINT material_versions_material_id_version_no_key UNIQUE (material_id, version_no);


--
-- Name: material_versions material_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_versions
    ADD CONSTRAINT material_versions_pkey PRIMARY KEY (id);


--
-- Name: materials materials_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.materials
    ADD CONSTRAINT materials_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_token_hash_key UNIQUE (token_hash);


--
-- Name: practice_items practice_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_items
    ADD CONSTRAINT practice_items_pkey PRIMARY KEY (id);


--
-- Name: practice_items practice_items_session_id_position_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_items
    ADD CONSTRAINT practice_items_session_id_position_key UNIQUE (session_id, "position");


--
-- Name: practice_items practice_items_session_id_question_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_items
    ADD CONSTRAINT practice_items_session_id_question_id_key UNIQUE (session_id, question_id);


--
-- Name: practice_sessions practice_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_sessions
    ADD CONSTRAINT practice_sessions_pkey PRIMARY KEY (id);


--
-- Name: question_ai_explanations question_ai_explanations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_ai_explanations
    ADD CONSTRAINT question_ai_explanations_pkey PRIMARY KEY (question_version_id);


--
-- Name: question_version_knowledge_points question_version_knowledge_points_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_version_knowledge_points
    ADD CONSTRAINT question_version_knowledge_points_pkey PRIMARY KEY (question_version_id, knowledge_point_id);


--
-- Name: question_versions question_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_versions
    ADD CONSTRAINT question_versions_pkey PRIMARY KEY (id);


--
-- Name: question_versions question_versions_question_id_version_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_versions
    ADD CONSTRAINT question_versions_question_id_version_no_key UNIQUE (question_id, version_no);


--
-- Name: questions questions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.questions
    ADD CONSTRAINT questions_pkey PRIMARY KEY (id);


--
-- Name: rate_limit_counters rate_limit_counters_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rate_limit_counters
    ADD CONSTRAINT rate_limit_counters_pkey PRIMARY KEY (key);


--
-- Name: source_sections source_sections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.source_sections
    ADD CONSTRAINT source_sections_pkey PRIMARY KEY (id);


--
-- Name: source_sections source_sections_source_id_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.source_sections
    ADD CONSTRAINT source_sections_source_id_name_key UNIQUE (source_id, name);


--
-- Name: sources sources_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sources
    ADD CONSTRAINT sources_pkey PRIMARY KEY (id);


--
-- Name: subjects subjects_exam_id_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subjects
    ADD CONSTRAINT subjects_exam_id_code_key UNIQUE (exam_id, code);


--
-- Name: subjects subjects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subjects
    ADD CONSTRAINT subjects_pkey PRIMARY KEY (id);


--
-- Name: user_answers user_answers_item_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_answers
    ADD CONSTRAINT user_answers_item_id_key UNIQUE (item_id);


--
-- Name: user_answers user_answers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_answers
    ADD CONSTRAINT user_answers_pkey PRIMARY KEY (id);


--
-- Name: user_knowledge_stats user_knowledge_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_knowledge_stats
    ADD CONSTRAINT user_knowledge_stats_pkey PRIMARY KEY (id);


--
-- Name: user_knowledge_stats user_knowledge_stats_user_id_knowledge_point_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_knowledge_stats
    ADD CONSTRAINT user_knowledge_stats_user_id_knowledge_point_id_key UNIQUE (user_id, knowledge_point_id);


--
-- Name: user_learning_memory user_learning_memory_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_learning_memory
    ADD CONSTRAINT user_learning_memory_pkey PRIMARY KEY (user_id);


--
-- Name: users users_email_normalized_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_normalized_key UNIQUE (email_normalized);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: idx_auth_sessions_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_auth_sessions_user ON public.auth_sessions USING btree (user_id);


--
-- Name: idx_grading_results_session; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_grading_results_session ON public.grading_results USING btree (session_id);


--
-- Name: idx_issue_reports_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_reports_status ON public.issue_reports USING btree (status, created_at);


--
-- Name: idx_jobs_claim; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_claim ON public.jobs USING btree (status, available_at);


--
-- Name: idx_knowledge_points_level_subject; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_points_level_subject ON public.knowledge_points USING btree (level_id, subject_id, status);


--
-- Name: idx_practice_items_visible_session; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_practice_items_visible_session ON public.practice_items USING btree (session_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_practice_sessions_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_practice_sessions_user ON public.practice_sessions USING btree (user_id, status, created_at);


--
-- Name: idx_practice_sessions_visible_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_practice_sessions_visible_user_created ON public.practice_sessions USING btree (user_id, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: idx_question_versions_level_subject; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_versions_level_subject ON public.question_versions USING btree (level_id, subject_id);


--
-- Name: idx_question_versions_source_order; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_versions_source_order ON public.question_versions USING btree (source_section_id, source_order);


--
-- Name: idx_questions_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_questions_status ON public.questions USING btree (status, published_version_id);


--
-- Name: uq_practice_sessions_submit_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_practice_sessions_submit_key ON public.practice_sessions USING btree (submit_key) WHERE (submit_key IS NOT NULL);


--
-- Name: ai_generated_question_answers ai_generated_question_answers_question_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_generated_question_answers
    ADD CONSTRAINT ai_generated_question_answers_question_version_id_fkey FOREIGN KEY (question_version_id) REFERENCES public.question_versions(id) ON DELETE CASCADE;


--
-- Name: answer_keys answer_keys_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.answer_keys
    ADD CONSTRAINT answer_keys_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: answer_keys answer_keys_question_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.answer_keys
    ADD CONSTRAINT answer_keys_question_version_id_fkey FOREIGN KEY (question_version_id) REFERENCES public.question_versions(id);


--
-- Name: audit_logs audit_logs_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id);


--
-- Name: auth_sessions auth_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auth_sessions
    ADD CONSTRAINT auth_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: exam_levels exam_levels_exam_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.exam_levels
    ADD CONSTRAINT exam_levels_exam_id_fkey FOREIGN KEY (exam_id) REFERENCES public.exams(id);


--
-- Name: materials fk_materials_current_version; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.materials
    ADD CONSTRAINT fk_materials_current_version FOREIGN KEY (current_version_id) REFERENCES public.material_versions(id);


--
-- Name: questions fk_questions_current_version; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.questions
    ADD CONSTRAINT fk_questions_current_version FOREIGN KEY (current_version_id) REFERENCES public.question_versions(id);


--
-- Name: questions fk_questions_published_version; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.questions
    ADD CONSTRAINT fk_questions_published_version FOREIGN KEY (published_version_id) REFERENCES public.question_versions(id);


--
-- Name: grading_results grading_results_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.grading_results
    ADD CONSTRAINT grading_results_item_id_fkey FOREIGN KEY (item_id) REFERENCES public.practice_items(id);


--
-- Name: grading_results grading_results_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.grading_results
    ADD CONSTRAINT grading_results_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.practice_sessions(id);


--
-- Name: import_items import_items_import_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_items
    ADD CONSTRAINT import_items_import_job_id_fkey FOREIGN KEY (import_job_id) REFERENCES public.import_jobs(id);


--
-- Name: import_items import_items_published_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_items
    ADD CONSTRAINT import_items_published_question_id_fkey FOREIGN KEY (published_question_id) REFERENCES public.questions(id);


--
-- Name: import_jobs import_jobs_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_jobs
    ADD CONSTRAINT import_jobs_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: issue_reports issue_reports_handled_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reports
    ADD CONSTRAINT issue_reports_handled_by_fkey FOREIGN KEY (handled_by) REFERENCES public.users(id);


--
-- Name: issue_reports issue_reports_practice_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reports
    ADD CONSTRAINT issue_reports_practice_item_id_fkey FOREIGN KEY (practice_item_id) REFERENCES public.practice_items(id);


--
-- Name: issue_reports issue_reports_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reports
    ADD CONSTRAINT issue_reports_question_id_fkey FOREIGN KEY (question_id) REFERENCES public.questions(id);


--
-- Name: issue_reports issue_reports_question_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reports
    ADD CONSTRAINT issue_reports_question_version_id_fkey FOREIGN KEY (question_version_id) REFERENCES public.question_versions(id);


--
-- Name: issue_reports issue_reports_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reports
    ADD CONSTRAINT issue_reports_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.practice_sessions(id);


--
-- Name: issue_reports issue_reports_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.issue_reports
    ADD CONSTRAINT issue_reports_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: knowledge_points knowledge_points_exam_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_points
    ADD CONSTRAINT knowledge_points_exam_id_fkey FOREIGN KEY (exam_id) REFERENCES public.exams(id);


--
-- Name: knowledge_points knowledge_points_level_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_points
    ADD CONSTRAINT knowledge_points_level_id_fkey FOREIGN KEY (level_id) REFERENCES public.exam_levels(id);


--
-- Name: knowledge_points knowledge_points_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_points
    ADD CONSTRAINT knowledge_points_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.knowledge_points(id);


--
-- Name: knowledge_points knowledge_points_subject_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_points
    ADD CONSTRAINT knowledge_points_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.subjects(id);


--
-- Name: material_versions material_versions_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_versions
    ADD CONSTRAINT material_versions_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: material_versions material_versions_material_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.material_versions
    ADD CONSTRAINT material_versions_material_id_fkey FOREIGN KEY (material_id) REFERENCES public.materials(id);


--
-- Name: materials materials_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.materials
    ADD CONSTRAINT materials_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: password_reset_tokens password_reset_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: practice_items practice_items_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_items
    ADD CONSTRAINT practice_items_question_id_fkey FOREIGN KEY (question_id) REFERENCES public.questions(id);


--
-- Name: practice_items practice_items_question_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_items
    ADD CONSTRAINT practice_items_question_version_id_fkey FOREIGN KEY (question_version_id) REFERENCES public.question_versions(id);


--
-- Name: practice_items practice_items_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_items
    ADD CONSTRAINT practice_items_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.practice_sessions(id);


--
-- Name: practice_sessions practice_sessions_level_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_sessions
    ADD CONSTRAINT practice_sessions_level_id_fkey FOREIGN KEY (level_id) REFERENCES public.exam_levels(id);


--
-- Name: practice_sessions practice_sessions_subject_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_sessions
    ADD CONSTRAINT practice_sessions_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.subjects(id);


--
-- Name: practice_sessions practice_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.practice_sessions
    ADD CONSTRAINT practice_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: question_ai_explanations question_ai_explanations_question_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_ai_explanations
    ADD CONSTRAINT question_ai_explanations_question_version_id_fkey FOREIGN KEY (question_version_id) REFERENCES public.question_versions(id);


--
-- Name: question_version_knowledge_points question_version_knowledge_points_knowledge_point_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_version_knowledge_points
    ADD CONSTRAINT question_version_knowledge_points_knowledge_point_id_fkey FOREIGN KEY (knowledge_point_id) REFERENCES public.knowledge_points(id);


--
-- Name: question_version_knowledge_points question_version_knowledge_points_question_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_version_knowledge_points
    ADD CONSTRAINT question_version_knowledge_points_question_version_id_fkey FOREIGN KEY (question_version_id) REFERENCES public.question_versions(id);


--
-- Name: question_versions question_versions_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_versions
    ADD CONSTRAINT question_versions_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: question_versions question_versions_level_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_versions
    ADD CONSTRAINT question_versions_level_id_fkey FOREIGN KEY (level_id) REFERENCES public.exam_levels(id);


--
-- Name: question_versions question_versions_material_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_versions
    ADD CONSTRAINT question_versions_material_version_id_fkey FOREIGN KEY (material_version_id) REFERENCES public.material_versions(id);


--
-- Name: question_versions question_versions_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_versions
    ADD CONSTRAINT question_versions_question_id_fkey FOREIGN KEY (question_id) REFERENCES public.questions(id);


--
-- Name: question_versions question_versions_source_section_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_versions
    ADD CONSTRAINT question_versions_source_section_id_fkey FOREIGN KEY (source_section_id) REFERENCES public.source_sections(id);


--
-- Name: question_versions question_versions_subject_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.question_versions
    ADD CONSTRAINT question_versions_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES public.subjects(id);


--
-- Name: questions questions_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.questions
    ADD CONSTRAINT questions_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: source_sections source_sections_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.source_sections
    ADD CONSTRAINT source_sections_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.sources(id) ON DELETE CASCADE;


--
-- Name: sources sources_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sources
    ADD CONSTRAINT sources_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: subjects subjects_exam_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subjects
    ADD CONSTRAINT subjects_exam_id_fkey FOREIGN KEY (exam_id) REFERENCES public.exams(id);


--
-- Name: user_answers user_answers_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_answers
    ADD CONSTRAINT user_answers_item_id_fkey FOREIGN KEY (item_id) REFERENCES public.practice_items(id);


--
-- Name: user_answers user_answers_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_answers
    ADD CONSTRAINT user_answers_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.practice_sessions(id);


--
-- Name: user_answers user_answers_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_answers
    ADD CONSTRAINT user_answers_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: user_knowledge_stats user_knowledge_stats_knowledge_point_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_knowledge_stats
    ADD CONSTRAINT user_knowledge_stats_knowledge_point_id_fkey FOREIGN KEY (knowledge_point_id) REFERENCES public.knowledge_points(id);


--
-- Name: user_knowledge_stats user_knowledge_stats_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_knowledge_stats
    ADD CONSTRAINT user_knowledge_stats_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: user_learning_memory user_learning_memory_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_learning_memory
    ADD CONSTRAINT user_learning_memory_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--



-- +goose Down
-- Baseline snapshot; no automatic down migration.
