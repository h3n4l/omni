-- Oracle DML statement corpus (INSERT, UPDATE, DELETE, MERGE)

-- @name: insert values
INSERT INTO employees (employee_id, first_name, last_name) VALUES (999, 'John', 'Doe')

-- @name: insert subquery
INSERT INTO emp_backup SELECT * FROM employees WHERE department_id = 10

-- @name: insert with returning
INSERT INTO employees (employee_id, first_name, last_name)
VALUES (998, 'Jane', 'Smith')
RETURNING employee_id INTO :id

-- @name: insert all
INSERT ALL
  INTO sales (prod_id, amount) VALUES (1, 100)
  INTO sales (prod_id, amount) VALUES (2, 200)
SELECT * FROM dual

-- @name: insert first conditional
INSERT FIRST
  WHEN salary > 10000 THEN INTO high_earners
  WHEN salary > 5000 THEN INTO mid_earners
  ELSE INTO low_earners
SELECT employee_id, salary FROM employees

-- @name: update simple
UPDATE employees SET salary = salary * 1.1 WHERE department_id = 10

-- @name: update multiple columns
UPDATE employees SET salary = 50000, commission_pct = 0.2
WHERE employee_id = 100

-- @name: update with subquery
UPDATE employees SET salary = (
  SELECT AVG(salary) FROM employees WHERE department_id = 20
)
WHERE department_id = 10

-- @name: update with returning
UPDATE employees SET salary = salary + 1000
WHERE employee_id = 100
RETURNING salary INTO :new_sal

-- @name: delete simple
DELETE FROM employees WHERE employee_id = 999

-- @name: delete with subquery
DELETE FROM employees
WHERE department_id IN (
  SELECT department_id FROM departments WHERE location_id = 2500
)

-- @name: delete with returning
DELETE FROM employees WHERE employee_id = 998
RETURNING first_name, last_name INTO :fname, :lname

-- @name: merge
MERGE INTO employee_bonuses b
USING (SELECT employee_id, salary FROM employees) e
ON (b.employee_id = e.employee_id)
WHEN MATCHED THEN UPDATE SET b.bonus = e.salary * 0.1
WHEN NOT MATCHED THEN INSERT (employee_id, bonus) VALUES (e.employee_id, e.salary * 0.1)
