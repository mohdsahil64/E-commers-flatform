-- Enable required PostgreSQL extensions for CARLITOS.
-- uuid-ossp / pgcrypto: UUID generation. pg_trgm: fuzzy search. citext: case-insensitive email.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "citext";
