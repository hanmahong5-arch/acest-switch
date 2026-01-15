-- ============================================================
-- CodeSwitch Multi-Database Initialization
-- Creates separate databases for each service
-- ============================================================

-- Create databases for each service
CREATE DATABASE casdoor;
CREATE DATABASE lago;

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE casdoor TO codeswitch;
GRANT ALL PRIVILEGES ON DATABASE lago TO codeswitch;

-- Create extensions in main database
\c codeswitch
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create extensions in casdoor database
\c casdoor
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create extensions in lago database
\c lago
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Switch back to main database
\c codeswitch
