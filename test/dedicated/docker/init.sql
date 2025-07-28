-- =========================================================
-- 1) Enable necessary extensions
-- =========================================================
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pgjwt";

-- =========================================================
-- 8) Role management
-- =========================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'web_anon') THEN
        CREATE ROLE web_anon NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'authenticated') THEN
        CREATE ROLE authenticated NOLOGIN;
    END IF;
    -- If you want to handle a separate DB role "postgres" for
    -- PostgREST, you can also ensure it exists:
    -- IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'postgres') THEN
    --     CREATE ROLE postgres NOLOGIN SUPERUSER;
    -- END IF;
END
$$;

-- =========================================================
-- 3) Create "users" table
-- =========================================================
CREATE TABLE users (
    id       SERIAL PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,    -- store the hashed password
    email    TEXT UNIQUE NOT NULL
);

-- Create the tasks table with an owner_id column
CREATE TABLE tasks (
    id SERIAL PRIMARY KEY,
    owner_id TEXT DEFAULT current_setting('request.jwt.claims', true)::json->>'sub',  -- user identifier (e.g., JWT "sub" claim)
    title TEXT NOT NULL,
    completed BOOLEAN DEFAULT FALSE
);

-- Enable Row-Level Security (RLS) on tasks
ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;

-- Create RLS policies so that a user can only see and modify their own tasks
-- Assumes the JWT token has a "sub" claim with the user id
CREATE POLICY tasks_select_policy ON tasks
  FOR SELECT
  USING (owner_id = current_setting('request.jwt.claims', true)::json->>'sub');

CREATE POLICY tasks_insert_policy ON tasks
  FOR INSERT
  WITH CHECK (
    owner_id = current_setting('request.jwt.claims', true)::json->>'sub'
    OR owner_id IS NULL
  );

CREATE POLICY tasks_update_policy ON tasks
  FOR UPDATE
  USING (owner_id = current_setting('request.jwt.claims', true)::json->>'sub')
  WITH CHECK (owner_id = current_setting('request.jwt.claims', true)::json->>'sub');

CREATE POLICY tasks_delete_policy ON tasks
  FOR DELETE
  USING (owner_id = current_setting('request.jwt.claims', true)::json->>'sub');


GRANT SELECT, INSERT, UPDATE, DELETE ON tasks TO authenticated;
GRANT SELECT, INSERT, UPDATE, DELETE ON tasks TO web_anon;

GRANT USAGE, SELECT, UPDATE ON SEQUENCE tasks_id_seq TO authenticated;
GRANT USAGE, SELECT, UPDATE ON SEQUENCE tasks_id_seq TO web_anon;


-- =========================================================
-- get_jwt_sub 
-- =========================================================

CREATE OR REPLACE FUNCTION get_jwt_sub()
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
    user_sub TEXT;
BEGIN
    -- Attempt to read the JWT claim "sub"
    user_sub := current_setting('request.jwt.claims', true)::json->>'sub';

    -- Return the result, or a message if it's null
    IF user_sub IS NULL THEN
        RETURN 'No sub claim found';
    END IF;

    RETURN user_sub;
END;
$$;

-- 2) Revoke all permissions from PUBLIC (nobody can call this by default)
REVOKE ALL ON FUNCTION get_jwt_sub() FROM PUBLIC;

-- 3) Grant execution to whichever database role PostgREST uses.
--    Typically web_anon or authenticated. The internal IF-check still restricts
--    actual usage to JWT tokens with role='postgres'.
GRANT EXECUTE ON FUNCTION get_jwt_sub() TO web_anon;
GRANT EXECUTE ON FUNCTION get_jwt_sub() TO authenticated;


CREATE OR REPLACE FUNCTION debug_jwt()
RETURNS TABLE (claim text, value text)
LANGUAGE sql
AS $$
  SELECT * FROM jsonb_each_text(current_setting('request.jwt.claims', true)::jsonb);
$$;

-- 2) Revoke all permissions from PUBLIC (nobody can call this by default)
REVOKE ALL ON FUNCTION debug_jwt() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION debug_jwt() TO web_anon;
GRANT EXECUTE ON FUNCTION debug_jwt() TO authenticated;

-- =========================================================
-- 9) Grant permissions to these roles
-- =========================================================
-- So PostgREST can access objects in schema "public"
GRANT USAGE ON SCHEMA public TO web_anon;
GRANT USAGE ON SCHEMA public TO authenticated;

-- 1) Create a minimal “dummy” function
CREATE OR REPLACE FUNCTION dummy_admin()
  RETURNS TEXT
  LANGUAGE plpgsql
  SECURITY DEFINER
AS $$
BEGIN
    -- Check the JWT claim
    IF current_setting('jwt.claims.role', true) <> 'postgres' THEN
        RAISE EXCEPTION 'Only the "postgres" role can call this function.';
    END IF;
    
    RETURN 'Success! You are the postgres user.';
END;
$$;

-- 2) Revoke all permissions from PUBLIC (nobody can call this by default)
REVOKE ALL ON FUNCTION dummy_admin() FROM PUBLIC;

-- 3) Grant execution to whichever database role PostgREST uses.
--    Typically web_anon or authenticated. The internal IF-check still restricts
--    actual usage to JWT tokens with role='postgres'.
GRANT EXECUTE ON FUNCTION dummy_admin() TO web_anon;
GRANT EXECUTE ON FUNCTION dummy_admin() TO authenticated;

-- Alternatively, if your PostgREST uses a single DB user (say "postgrest"), just do:
-- GRANT EXECUTE ON FUNCTION dummy_admin() TO postgrest;


-- =========================================================
-- 6) Superuser-only function to create new users
-- =========================================================
CREATE OR REPLACE FUNCTION create_user(
    p_username TEXT,
    p_password TEXT,
    p_email    TEXT
)
RETURNS VOID
LANGUAGE plpgsql
SECURITY DEFINER
AS
$$
BEGIN
    /*
      Only allow execution if the JWT claim "role" is 'postgres'.
      This check ensures only the superuser token can call this.
    */
    IF current_setting('jwt.claims.role', true) <> 'postgres' THEN
        RAISE EXCEPTION 'Only a "postgres" (superuser) JWT can create new users.';
    END IF;

    INSERT INTO users (username, password, email)
    VALUES (
        p_username,
        crypt(p_password, gen_salt('bf')),
        p_email
    );
END;
$$;

-- Revoke from PUBLIC, then grant to whichever role you want to allow
REVOKE ALL ON FUNCTION create_user(p_username TEXT, p_password TEXT, p_email TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION create_user(p_username TEXT, p_password TEXT, p_email TEXT) TO web_anon;
GRANT EXECUTE ON FUNCTION create_user(p_username TEXT, p_password TEXT, p_email TEXT) TO authenticated;




-- =========================================================
-- 7) Login function to generate a JWT token
-- =========================================================
CREATE OR REPLACE FUNCTION login_user(
    p_password TEXT,
    p_username TEXT
)
RETURNS TEXT
LANGUAGE plpgsql
SECURITY DEFINER
AS
$$
DECLARE
    user_record   users%ROWTYPE;
    token         TEXT;
    user_role     TEXT;
BEGIN
    SELECT * INTO user_record
      FROM users
     WHERE username = p_username;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Invalid username or password';
    END IF;

    -- Compare hashed password
    IF user_record.password <> crypt(p_password, user_record.password) THEN
        RAISE EXCEPTION 'Invalid username or password';
    END IF;

    -- Decide the role for the JWT:
    IF user_record.username = 'postgres' THEN
        user_role := 'postgres';
    ELSE
        user_role := 'authenticated';
    END IF;

    -- Sign the JWT (change secret to a secure, lengthy phrase)
    token := sign(
        json_build_object(
            'sub',  user_record.id::TEXT,  -- user identifier
            'role', user_role
        )::json,
        'a-string-secret-at-least-256-bits-long'
    );

    RETURN token;
END;
$$;

-- Revoke from PUBLIC, then grant to roles that should be able to login
REVOKE ALL ON FUNCTION login_user(p_password TEXT, p_username TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION login_user(p_password TEXT, p_username TEXT) TO web_anon;
GRANT EXECUTE ON FUNCTION login_user(p_password TEXT, p_username TEXT) TO authenticated;