-- Oracle DDL statement corpus (CREATE, ALTER, DROP)

-- @name: create table simple
CREATE TABLE test_table (
  id NUMBER PRIMARY KEY,
  name VARCHAR2(100) NOT NULL,
  created_date DATE DEFAULT SYSDATE
)

-- @name: create table with constraints
CREATE TABLE orders (
  order_id NUMBER,
  customer_id NUMBER NOT NULL,
  order_date DATE,
  amount NUMBER(10,2),
  CONSTRAINT pk_orders PRIMARY KEY (order_id),
  CONSTRAINT fk_customer FOREIGN KEY (customer_id) REFERENCES customers(customer_id),
  CONSTRAINT chk_amount CHECK (amount > 0)
)

-- @name: create table as select
CREATE TABLE emp_backup AS SELECT * FROM employees WHERE department_id = 10

-- @name: create global temporary table
CREATE GLOBAL TEMPORARY TABLE temp_results (
  id NUMBER,
  result VARCHAR2(200)
) ON COMMIT DELETE ROWS

-- @name: create index
CREATE INDEX idx_emp_dept ON employees(department_id)

-- @name: create unique index
CREATE UNIQUE INDEX uk_emp_email ON employees(email)

-- @name: create bitmap index
CREATE BITMAP INDEX bx_emp_dept ON employees(department_id)

-- @name: create view
CREATE VIEW emp_view AS
SELECT employee_id, first_name, last_name, department_id FROM employees

-- @name: create or replace view
CREATE OR REPLACE VIEW dept_summary AS
SELECT d.department_id, d.department_name, COUNT(e.employee_id) AS emp_count
FROM departments d LEFT JOIN employees e ON d.department_id = e.department_id
GROUP BY d.department_id, d.department_name

-- @name: create materialized view
CREATE MATERIALIZED VIEW mv_dept_salary
BUILD IMMEDIATE
REFRESH COMPLETE ON DEMAND
AS SELECT department_id, SUM(salary) AS total_sal FROM employees GROUP BY department_id

-- @name: create sequence
CREATE SEQUENCE emp_seq START WITH 1000 INCREMENT BY 1 NOCACHE NOCYCLE

-- @name: create synonym
CREATE SYNONYM emp FOR hr.employees

-- @name: create public synonym
CREATE PUBLIC SYNONYM all_employees FOR hr.employees

-- @name: create type object
CREATE TYPE address_type AS OBJECT (
  street VARCHAR2(200),
  city VARCHAR2(100),
  state VARCHAR2(50),
  zip VARCHAR2(20)
)

-- @name: create type collection
CREATE TYPE number_list AS TABLE OF NUMBER

-- @name: create type varray
CREATE TYPE phone_list AS VARRAY(5) OF VARCHAR2(20)

-- @name: alter table add column
ALTER TABLE employees ADD (middle_name VARCHAR2(50))

-- @name: alter table modify column
ALTER TABLE employees MODIFY (first_name VARCHAR2(200))

-- @name: alter table drop column
ALTER TABLE employees DROP COLUMN middle_name

-- @name: alter table add constraint
ALTER TABLE employees ADD CONSTRAINT uk_email UNIQUE (email)

-- @name: alter table drop constraint
ALTER TABLE employees DROP CONSTRAINT uk_email

-- @name: alter table rename column
ALTER TABLE employees RENAME COLUMN first_name TO given_name

-- @name: alter table rename
ALTER TABLE old_table RENAME TO new_table

-- @name: alter index rebuild
ALTER INDEX idx_emp_dept REBUILD

-- @name: drop table
DROP TABLE test_table

-- @name: drop table cascade constraints
DROP TABLE orders CASCADE CONSTRAINTS

-- @name: drop table purge
DROP TABLE temp_data PURGE

-- @name: drop index
DROP INDEX idx_emp_dept

-- @name: drop view
DROP VIEW emp_view

-- @name: drop sequence
DROP SEQUENCE emp_seq

-- @name: drop synonym
DROP SYNONYM emp

-- @name: drop type
DROP TYPE address_type

-- @name: truncate table
TRUNCATE TABLE temp_results

-- @name: comment on table
COMMENT ON TABLE employees IS 'Employee master table'

-- @name: comment on column
COMMENT ON COLUMN employees.salary IS 'Annual salary in USD'
