-- @name: simple select
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT 1

-- @name: select from dual
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT 1 FROM dual

-- @name: select with alias
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT 1 AS one, 'hello' AS greeting

-- @name: select from table
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id, name, email FROM users

-- @name: select star
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM users

-- @name: select distinct
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT DISTINCT department_id FROM employees

-- @name: select with where
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE department_id = 10

-- @name: select with and/or
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE salary > 5000 AND department_id IN (10, 20, 30)

-- @name: select with between
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE salary BETWEEN 3000 AND 8000

-- @name: select with like
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE name LIKE 'John%'

-- @name: select with is null
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE manager_id IS NULL

-- @name: select with is not null
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE email IS NOT NULL

-- @name: select with inner join
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT e.id, d.name
FROM employees e
INNER JOIN departments d ON e.department_id = d.id

-- @name: select with left join
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT e.id, d.name
FROM employees e
LEFT JOIN departments d ON e.department_id = d.id

-- @name: select with right join
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT e.id, d.name
FROM employees e
RIGHT JOIN departments d ON e.department_id = d.id

-- @name: select with cross join
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM t1 CROSS JOIN t2

-- @name: select with natural join
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM t1 NATURAL JOIN t2

-- @name: select with join using
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM t1 JOIN t2 USING (id)

-- @name: select with self join
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT e.name, m.name AS manager
FROM employees e
LEFT JOIN employees m ON e.manager_id = m.id

-- @name: select with group by
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT department_id, COUNT(*) AS cnt
FROM employees
GROUP BY department_id

-- @name: select with group by having
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT department_id, AVG(salary) AS avg_sal
FROM employees
GROUP BY department_id
HAVING AVG(salary) > 5000

-- @name: select with order by
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id, salary FROM employees ORDER BY salary DESC, id ASC

-- @name: select with limit
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees ORDER BY id LIMIT 10

-- @name: select with limit offset
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees ORDER BY id LIMIT 10 OFFSET 20

-- @name: select with limit comma syntax
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees ORDER BY id LIMIT 20, 10

-- @name: select with subquery in where
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE department_id IN (
  SELECT id FROM departments WHERE location = 'NYC'
)

-- @name: select with correlated subquery
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT e.id, e.salary FROM employees e
WHERE e.salary > (SELECT AVG(salary) FROM employees WHERE department_id = e.department_id)

-- @name: select with exists
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM departments d
WHERE EXISTS (SELECT 1 FROM employees e WHERE e.department_id = d.id)

-- @name: select with derived table
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM (
  SELECT id, salary, ROW_NUMBER() OVER (ORDER BY salary DESC) AS rn
  FROM employees
) AS ranked WHERE rn <= 10

-- @name: select with CTE
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
WITH dept_avg AS (
  SELECT department_id, AVG(salary) AS avg_sal FROM employees GROUP BY department_id
)
SELECT e.id, e.salary, da.avg_sal
FROM employees e
JOIN dept_avg da ON e.department_id = da.department_id

-- @name: select with recursive CTE
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
WITH RECURSIVE org AS (
  SELECT id, manager_id, name, 1 AS lvl FROM employees WHERE manager_id IS NULL
  UNION ALL
  SELECT e.id, e.manager_id, e.name, o.lvl + 1
  FROM employees e JOIN org o ON e.manager_id = o.id
)
SELECT * FROM org

-- @name: select with case expression
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id,
  CASE WHEN salary > 10000 THEN 'HIGH'
       WHEN salary > 5000 THEN 'MID'
       ELSE 'LOW'
  END AS salary_band
FROM employees

-- @name: select with simple case
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id,
  CASE status WHEN 'A' THEN 'Active' WHEN 'I' THEN 'Inactive' ELSE 'Unknown' END AS label
FROM users

-- @name: select with window function row_number
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id, salary,
  ROW_NUMBER() OVER (PARTITION BY department_id ORDER BY salary DESC) AS rn
FROM employees

-- @name: select with window function rank
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id, salary,
  RANK() OVER (ORDER BY salary DESC) AS salary_rank,
  DENSE_RANK() OVER (ORDER BY salary DESC) AS dense_rank
FROM employees

-- @name: select with window frame
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id, salary,
  SUM(salary) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING) AS moving_sum
FROM employees

-- @name: select with union
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id, name FROM employees WHERE department_id = 10
UNION
SELECT id, name FROM employees WHERE department_id = 20

-- @name: select with union all
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id FROM employees WHERE salary > 10000
UNION ALL
SELECT id FROM employees WHERE department_id = 10

-- @name: select with intersect
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id FROM employees WHERE salary > 10000
INTERSECT
SELECT id FROM employees WHERE department_id = 10

-- @name: select with except
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT id FROM employees WHERE department_id = 10
EXCEPT
SELECT id FROM employees WHERE salary < 5000

-- @name: select for update
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE department_id = 10 FOR UPDATE

-- @name: select lock in share mode
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE id = 100 LOCK IN SHARE MODE

-- @name: select for share
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees WHERE id = 100 FOR SHARE

-- @name: select with group_concat
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT department_id,
  GROUP_CONCAT(name ORDER BY name SEPARATOR ', ') AS names
FROM employees
GROUP BY department_id

-- @name: select with aggregate functions
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT COUNT(*), SUM(salary), AVG(salary), MIN(salary), MAX(salary)
FROM employees

-- @name: select with count distinct
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT COUNT(DISTINCT department_id) FROM employees

-- @name: select with cast
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT CAST(salary AS CHAR), CAST(hire_date AS DATE) FROM employees

-- @name: select with convert
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT CONVERT(name, CHAR) FROM employees

-- @name: select with coalesce
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT COALESCE(phone, email, 'N/A') FROM employees

-- @name: select with if
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT IF(salary > 5000, 'high', 'low') FROM employees

-- @name: select with ifnull
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT IFNULL(manager_id, 0) FROM employees

-- @name: select with string functions
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT CONCAT(first_name, ' ', last_name), UPPER(name), LOWER(name), LENGTH(name)
FROM employees

-- @name: select with date functions
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT NOW(), CURDATE(), DATE_FORMAT(hire_date, '%Y-%m-%d') FROM employees

-- @name: select with interval
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM events WHERE created_at > NOW() - INTERVAL 7 DAY

-- @name: select high_priority
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT HIGH_PRIORITY * FROM employees

-- @name: select sql_calc_found_rows
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT SQL_CALC_FOUND_ROWS * FROM employees LIMIT 10

-- @name: select into outfile
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT * FROM employees INTO OUTFILE '/tmp/employees.csv'

-- @name: select with multiple joins
-- @valid: true
-- @source: MySQL 8.0 Reference Manual
SELECT e.name, d.name, l.city
FROM employees e
JOIN departments d ON e.department_id = d.id
JOIN locations l ON d.location_id = l.id
WHERE l.country = 'US'

-- @name: missing FROM with WHERE
-- @valid: false
-- @source: synthetic
SELECT * WHERE 1=1

-- @name: double WHERE clause
-- @valid: false
-- @source: synthetic
SELECT * FROM t WHERE 1=1 WHERE 2=2

-- @name: unclosed parenthesis
-- @valid: false
-- @source: synthetic
SELECT (1 + 2 FROM t

-- @name: missing expression after AND
-- @valid: false
-- @source: synthetic
SELECT * FROM t WHERE a = 1 AND

-- @name: invalid join syntax
-- @valid: false
-- @source: synthetic
SELECT * FROM t1 JOIN ON t2

-- @name: trailing garbage after valid statement — actually valid implicit alias
-- @valid: true
-- @source: SCENARIOS-mysql-strict.md section 1.1
SELECT 1 GARBAGE

-- @name: trailing keyword after valid statement
-- @valid: false
-- @source: SCENARIOS-mysql-strict.md section 1.1
SELECT 1 FROM

-- @name: trailing identifier after valid statement — actually valid implicit alias
-- @valid: true
-- @source: SCENARIOS-mysql-strict.md section 1.1
SELECT 1 abc

-- @name: trailing operator after valid statement
-- @valid: false
-- @source: SCENARIOS-mysql-strict.md section 1.1
SELECT 1 +

-- @name: trailing parenthesis after valid statement
-- @valid: false
-- @source: SCENARIOS-mysql-strict.md section 1.1
SELECT 1 )

-- @name: trailing garbage in multi-statement — actually valid implicit alias
-- @valid: true
-- @source: SCENARIOS-mysql-strict.md section 1.1
SELECT 1; SELECT 2 GARBAGE

-- @name: multiple semicolon-separated statements
-- @valid: true
-- @source: SCENARIOS-mysql-strict.md section 1.1
SELECT 1; SELECT 2

-- @name: trailing semicolon
-- @valid: true
-- @source: SCENARIOS-mysql-strict.md section 1.1
SELECT 1;

-- @name: single semicolon
-- @valid: true
-- @source: SCENARIOS-mysql-strict.md section 1.1
;
