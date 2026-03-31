-- Oracle administrative and security statements

-- @name: grant select
GRANT SELECT ON employees TO hr_user

-- @name: grant multiple
GRANT SELECT, INSERT, UPDATE ON employees TO app_user

-- @name: grant with grant option
GRANT SELECT ON employees TO hr_manager WITH GRANT OPTION

-- @name: grant role
GRANT dba TO admin_user

-- @name: revoke select
REVOKE SELECT ON employees FROM hr_user

-- @name: create user
CREATE USER test_user IDENTIFIED BY password123

-- @name: alter user
ALTER USER test_user IDENTIFIED BY new_password

-- @name: drop user
DROP USER test_user CASCADE

-- @name: create role
CREATE ROLE app_role

-- @name: create profile
CREATE PROFILE secure_profile LIMIT
  FAILED_LOGIN_ATTEMPTS 3
  PASSWORD_LOCK_TIME 1

-- @name: alter session set
ALTER SESSION SET NLS_DATE_FORMAT = 'YYYY-MM-DD'

-- @name: set transaction
SET TRANSACTION READ ONLY

-- @name: commit
COMMIT

-- @name: rollback
ROLLBACK

-- @name: savepoint
SAVEPOINT sp1

-- @name: rollback to savepoint
ROLLBACK TO SAVEPOINT sp1

-- @name: explain plan
EXPLAIN PLAN FOR SELECT * FROM employees

-- @name: flashback table
FLASHBACK TABLE employees TO TIMESTAMP SYSTIMESTAMP - INTERVAL '1' HOUR

-- @name: purge recyclebin
PURGE RECYCLEBIN

-- @name: analyze table
ANALYZE TABLE employees COMPUTE STATISTICS

-- @name: create tablespace
CREATE TABLESPACE app_data
DATAFILE '/u01/app_data01.dbf' SIZE 100M
AUTOEXTEND ON NEXT 50M

-- @name: alter tablespace
ALTER TABLESPACE app_data ADD DATAFILE '/u01/app_data02.dbf' SIZE 100M

-- @name: create database link
CREATE DATABASE LINK remote_db
CONNECT TO remote_user IDENTIFIED BY remote_pass
USING 'remote_tns'
