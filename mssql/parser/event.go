// Package parser - event.go implements T-SQL EVENT NOTIFICATION and EVENT SESSION statement parsing.
package parser

import (
	"strings"

	nodes "github.com/bytebase/omni/mssql/ast"
)

// parseCreateEventNotificationStmt parses CREATE EVENT NOTIFICATION.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-event-notification-transact-sql
//
//	CREATE EVENT NOTIFICATION event_notification_name
//	ON { SERVER | DATABASE | QUEUE queue_name }
//	[ WITH FAN_IN ]
//	FOR { event_type | event_group } [ , ...n ]
//	TO SERVICE 'broker_service' , { 'broker_instance_specifier' | 'current database' }
func (p *Parser) parseCreateEventNotificationStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EVENT NOTIFICATION keywords already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "CREATE",
		ObjectType: "EVENT NOTIFICATION",
		Loc:        nodes.Loc{Start: loc},
	}

	// event_notification_name
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// Consume rest of statement (ON, WITH FAN_IN, FOR, TO SERVICE clauses)
	stmt.Options = p.parseEventNotificationOptions()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseDropEventNotificationStmt parses DROP EVENT NOTIFICATION.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/drop-event-notification-transact-sql
//
//	DROP EVENT NOTIFICATION notification_name [ ,...n ]
//	ON { SERVER | DATABASE | QUEUE queue_name }
func (p *Parser) parseDropEventNotificationStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EVENT NOTIFICATION keywords already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "DROP",
		ObjectType: "EVENT NOTIFICATION",
		Loc:        nodes.Loc{Start: loc},
	}

	// notification_name [ ,...n ]
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// Consume rest: optional additional names and ON clause
	stmt.Options = p.parseEventNotificationOptions()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseEventNotificationOptions consumes ON/WITH/FOR/TO clauses of EVENT NOTIFICATION.
func (p *Parser) parseEventNotificationOptions() *nodes.List {
	var opts []nodes.Node

	for p.cur.Type != ';' && p.cur.Type != tokEOF && p.cur.Type != kwGO {
		switch {
		case p.cur.Type == kwON:
			p.advance()
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "SERVER") {
				opts = append(opts, &nodes.String{Str: "ON=SERVER"})
				p.advance()
			} else if p.cur.Type == kwDATABASE {
				opts = append(opts, &nodes.String{Str: "ON=DATABASE"})
				p.advance()
			} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "QUEUE") {
				p.advance()
				queueName := ""
				if p.isIdentLike() || p.cur.Type == tokSCONST {
					queueName = p.cur.Str
					p.advance()
				}
				// Qualified name (schema.queue)
				for p.cur.Type == '.' {
					p.advance()
					if p.isIdentLike() || p.cur.Type == tokSCONST {
						queueName += "." + p.cur.Str
						p.advance()
					}
				}
				opts = append(opts, &nodes.String{Str: "ON=QUEUE " + queueName})
			}
		case p.cur.Type == kwWITH:
			p.advance()
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "FAN_IN") {
				opts = append(opts, &nodes.String{Str: "FAN_IN"})
				p.advance()
			}
		case p.cur.Type == kwFOR:
			p.advance()
			// Collect event types/groups
			for {
				if p.isIdentLike() || p.cur.Type == tokSCONST {
					opts = append(opts, &nodes.String{Str: "FOR=" + strings.ToUpper(p.cur.Str)})
					p.advance()
				}
				if _, ok := p.match(','); !ok {
					break
				}
			}
		case p.cur.Type == kwTO:
			p.advance()
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "SERVICE") {
				p.advance()
			}
			// 'broker_service'
			if p.cur.Type == tokSCONST {
				opts = append(opts, &nodes.String{Str: "SERVICE=" + p.cur.Str})
				p.advance()
			}
			p.match(',')
			// 'broker_instance_specifier' or 'current database'
			if p.cur.Type == tokSCONST {
				opts = append(opts, &nodes.String{Str: "INSTANCE=" + p.cur.Str})
				p.advance()
			}
		default:
			// Skip commas between notification names for DROP
			if p.cur.Type == ',' {
				p.advance()
				if p.isIdentLike() || p.cur.Type == tokSCONST {
					opts = append(opts, &nodes.String{Str: "NAME=" + p.cur.Str})
					p.advance()
				}
			} else {
				p.advance()
			}
		}
	}

	if len(opts) == 0 {
		return nil
	}
	return &nodes.List{Items: opts}
}

// parseCreateEventSessionStmt parses CREATE EVENT SESSION.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-event-session-transact-sql
//
//	CREATE EVENT SESSION event_session_name
//	ON { SERVER | DATABASE }
//	{
//	    <event_definition> [ , ...n ]
//	    [ <event_target_definition> [ , ...n ] ]
//	    [ WITH ( <event_session_options> [ , ...n ] ) ]
//	}
//
//	<event_definition>::=
//	    ADD EVENT [event_module_guid].event_package_name.event_name
//	         [ ( {
//	                 [ SET { event_customizable_attribute = <value> [ , ...n ] } ]
//	                 [ ACTION ( { [event_module_guid].event_package_name.action_name [ , ...n ] } ) ]
//	                 [ WHERE <predicate_expression> ]
//	        } ) ]
//
//	<event_target_definition>::=
//	    ADD TARGET [event_module_guid].event_package_name.target_name
//	        [ ( SET { target_parameter_name = <value> [ , ...n ] } ) ]
//
//	<event_session_options>::=
//	    [       MAX_MEMORY = size [ KB | MB ] ]
//	    [ [ , ] EVENT_RETENTION_MODE = { ALLOW_SINGLE_EVENT_LOSS | ALLOW_MULTIPLE_EVENT_LOSS | NO_EVENT_LOSS } ]
//	    [ [ , ] MAX_DISPATCH_LATENCY = { seconds SECONDS | INFINITE } ]
//	    [ [ , ] MAX_EVENT_SIZE = size [ KB | MB ] ]
//	    [ [ , ] MEMORY_PARTITION_MODE = { NONE | PER_NODE | PER_CPU } ]
//	    [ [ , ] TRACK_CAUSALITY = { ON | OFF } ]
//	    [ [ , ] STARTUP_STATE = { ON | OFF } ]
func (p *Parser) parseCreateEventSessionStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EVENT SESSION keywords already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "CREATE",
		ObjectType: "EVENT SESSION",
		Loc:        nodes.Loc{Start: loc},
	}

	// event_session_name
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// Consume rest of statement (ON, ADD EVENT, ADD TARGET, WITH)
	stmt.Options = p.parseEventSessionBody()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterEventSessionStmt parses ALTER EVENT SESSION.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-event-session-transact-sql
//
//	ALTER EVENT SESSION event_session_name
//	ON { SERVER | DATABASE }
//	{
//	    [ { <add_drop_event> [ , ...n ] }
//	       | { <add_drop_event_target> [ , ...n ] } ]
//	    [ WITH ( <event_session_options> [ , ...n ] ) ]
//	    ]
//	    | [ STATE = { START | STOP } ]
//	}
//
//	<add_drop_event>::=
//	    ADD EVENT <event_specifier> [ ( ... ) ]
//	    | DROP EVENT <event_specifier>
//
//	<add_drop_event_target>::=
//	    ADD TARGET <event_target_specifier> [ ( SET ... ) ]
//	    | DROP TARGET <event_target_specifier>
func (p *Parser) parseAlterEventSessionStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EVENT SESSION keywords already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "ALTER",
		ObjectType: "EVENT SESSION",
		Loc:        nodes.Loc{Start: loc},
	}

	// event_session_name
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// Consume rest of statement
	stmt.Options = p.parseEventSessionBody()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseDropEventSessionStmt parses DROP EVENT SESSION.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/drop-event-session-transact-sql
//
//	DROP EVENT SESSION event_session_name
//	ON { SERVER | DATABASE }
func (p *Parser) parseDropEventSessionStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EVENT SESSION keywords already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "DROP",
		ObjectType: "EVENT SESSION",
		Loc:        nodes.Loc{Start: loc},
	}

	// event_session_name
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// ON { SERVER | DATABASE }
	stmt.Options = p.parseEventSessionBody()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseEventSessionBody consumes ON, ADD EVENT/TARGET, DROP EVENT/TARGET, WITH, STATE clauses.
func (p *Parser) parseEventSessionBody() *nodes.List {
	var opts []nodes.Node

	// ON { SERVER | DATABASE }
	if p.cur.Type == kwON {
		p.advance()
		if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "SERVER") {
			opts = append(opts, &nodes.String{Str: "ON=SERVER"})
			p.advance()
		} else if p.cur.Type == kwDATABASE {
			opts = append(opts, &nodes.String{Str: "ON=DATABASE"})
			p.advance()
		}
	}

	// Rest of body: ADD EVENT, ADD TARGET, DROP EVENT, DROP TARGET, STATE, WITH
	for p.cur.Type != ';' && p.cur.Type != tokEOF && p.cur.Type != kwGO {
		if p.cur.Type == kwADD {
			p.advance()
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "EVENT") {
				p.advance()
				opts = append(opts, p.parseEventSessionEventSpec("ADD EVENT")...)
			} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "TARGET") {
				p.advance()
				opts = append(opts, p.parseEventSessionTargetSpec("ADD TARGET")...)
			}
			// Check for comma then another ADD
			if p.cur.Type == ',' {
				p.advance()
				continue
			}
		} else if p.cur.Type == kwDROP {
			p.advance()
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "EVENT") {
				p.advance()
				eventName := p.parseEventSessionDottedName()
				opts = append(opts, &nodes.String{Str: "DROP EVENT=" + eventName})
			} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "TARGET") {
				p.advance()
				targetName := p.parseEventSessionDottedName()
				opts = append(opts, &nodes.String{Str: "DROP TARGET=" + targetName})
			}
			// Check for comma then another DROP/ADD
			if p.cur.Type == ',' {
				p.advance()
				continue
			}
		} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "STATE") {
			p.advance()
			p.match('=')
			if p.isIdentLike() {
				opts = append(opts, &nodes.String{Str: "STATE=" + strings.ToUpper(p.cur.Str)})
				p.advance()
			}
		} else if p.cur.Type == kwWITH {
			p.advance()
			if p.cur.Type == '(' {
				p.advance()
				for p.cur.Type != ')' && p.cur.Type != tokEOF {
					if p.isIdentLike() || p.cur.Type == kwON || p.cur.Type == kwOFF {
						optName := strings.ToUpper(p.cur.Str)
						p.advance()
						if p.cur.Type == '=' {
							p.advance()
							// Value could be: number [KB|MB|SECONDS], identifier, ON/OFF
							val := ""
							if p.isIdentLike() || p.cur.Type == tokICONST || p.cur.Type == tokSCONST ||
								p.cur.Type == kwON || p.cur.Type == kwOFF {
								val = strings.ToUpper(p.cur.Str)
								p.advance()
							}
							// Check for unit suffix (KB, MB, SECONDS, etc.)
							if p.isIdentLike() && (matchesKeywordCI(p.cur.Str, "KB") || matchesKeywordCI(p.cur.Str, "MB") ||
								matchesKeywordCI(p.cur.Str, "SECONDS") || matchesKeywordCI(p.cur.Str, "MINUTES") ||
								matchesKeywordCI(p.cur.Str, "HOURS") || matchesKeywordCI(p.cur.Str, "DAYS")) {
								val += " " + strings.ToUpper(p.cur.Str)
								p.advance()
							}
							optName += "=" + val
						}
						opts = append(opts, &nodes.String{Str: optName})
					} else {
						p.advance()
					}
					p.match(',')
				}
				p.match(')')
			}
		} else {
			break
		}
	}

	if len(opts) == 0 {
		return nil
	}
	return &nodes.List{Items: opts}
}

// parseEventSessionEventSpec parses an ADD EVENT specifier with optional (SET/ACTION/WHERE).
func (p *Parser) parseEventSessionEventSpec(prefix string) []nodes.Node {
	var result []nodes.Node

	eventName := p.parseEventSessionDottedName()
	result = append(result, &nodes.String{Str: prefix + "=" + eventName})

	// Optional parenthesized SET/ACTION/WHERE
	if p.cur.Type == '(' {
		p.advance()
		depth := 1
		for depth > 0 && p.cur.Type != tokEOF {
			if p.cur.Type == '(' {
				depth++
			} else if p.cur.Type == ')' {
				depth--
			}
			if depth > 0 {
				p.advance()
			}
		}
		p.match(')')
	}

	return result
}

// parseEventSessionTargetSpec parses an ADD TARGET specifier with optional (SET ...).
func (p *Parser) parseEventSessionTargetSpec(prefix string) []nodes.Node {
	var result []nodes.Node

	targetName := p.parseEventSessionDottedName()
	result = append(result, &nodes.String{Str: prefix + "=" + targetName})

	// Optional parenthesized SET
	if p.cur.Type == '(' {
		p.advance()
		depth := 1
		for depth > 0 && p.cur.Type != tokEOF {
			if p.cur.Type == '(' {
				depth++
			} else if p.cur.Type == ')' {
				depth--
			}
			if depth > 0 {
				p.advance()
			}
		}
		p.match(')')
	}

	return result
}

// parseEventSessionDottedName consumes a dotted name like package.event_name or [guid].package.name.
func (p *Parser) parseEventSessionDottedName() string {
	name := ""
	if p.isIdentLike() || p.cur.Type == tokSCONST || p.cur.Type == tokICONST {
		name = p.cur.Str
		p.advance()
	}
	for p.cur.Type == '.' {
		p.advance()
		if p.isIdentLike() || p.cur.Type == tokSCONST || p.cur.Type == tokICONST {
			name += "." + p.cur.Str
			p.advance()
		}
	}
	return name
}
