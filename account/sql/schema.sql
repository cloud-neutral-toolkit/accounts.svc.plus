--
-- PostgreSQL database dump
--

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;

--
-- Name: set_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

--
-- Name: identities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.identities (
    uuid uuid DEFAULT gen_random_uuid() NOT NULL,
    user_uuid uuid NOT NULL,
    provider text NOT NULL,
    external_id text NOT NULL
);

--
-- Name: sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sessions (
    uuid uuid DEFAULT gen_random_uuid() NOT NULL,
    user_uuid uuid NOT NULL,
    token text NOT NULL,
    expires_at timestamp with time zone NOT NULL
);

--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    uuid uuid DEFAULT gen_random_uuid() NOT NULL,
    username text NOT NULL,
    password text NOT NULL,
    email text,
    email_verified_at timestamp with time zone,
    mfa_totp_secret text,
    mfa_enabled boolean DEFAULT false NOT NULL,
    mfa_secret_issued_at timestamp with time zone,
    mfa_confirmed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now(),

    updated_at timestamp with time zone DEFAULT now(),

    email_verified boolean GENERATED ALWAYS AS (email_verified_at IS NOT NULL) STORED
);

--
-- Name: idx_identities_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_identities_provider ON public.identities USING btree (provider);

--
-- Name: idx_identities_user_uuid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_identities_user_uuid ON public.identities USING btree (user_uuid);

--
-- Name: idx_sessions_user_uuid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sessions_user_uuid ON public.sessions USING btree (user_uuid);

--
-- Name: identities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.identities
    ADD CONSTRAINT identities_pkey PRIMARY KEY (uuid);

--
-- Name: identities_provider_external_id_uk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.identities
    ADD CONSTRAINT identities_provider_external_id_uk UNIQUE (provider, external_id);

--
-- Name: sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (uuid);

--
-- Name: users_email_uk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_uk UNIQUE (email);

--
-- Name: users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (uuid);

--
-- Name: users_username_uk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_uk UNIQUE (username);

--
-- Name: identities_user_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.identities
    ADD CONSTRAINT identities_user_fk FOREIGN KEY (user_uuid) REFERENCES public.users(uuid) ON DELETE CASCADE;

--
-- Name: sessions_user_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_user_fk FOREIGN KEY (user_uuid) REFERENCES public.users(uuid) ON DELETE CASCADE;

--
-- Name: trg_users_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_users_set_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

