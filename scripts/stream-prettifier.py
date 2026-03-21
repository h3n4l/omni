import sys
import json

def prettify():
    for line in sys.stdin:
        if not line.strip():
            continue
        try:
            event = json.loads(line)
            event_type = event.get("event")
            
            if event_type == "message":
                # Print streaming text content
                text = event.get("data", {}).get("text", "")
                sys.stdout.write(text)
                sys.stdout.flush()
            
            elif event_type == "tool_use":
                # Print tool calls
                tool = event.get("data", {}).get("name")
                args = event.get("data", {}).get("arguments", {})
                sys.stdout.write(f"\n[Tool Use: {tool}({json.dumps(args)})]\n")
                sys.stdout.flush()
            
            elif event_type == "tool_result":
                # Optionally print tool results (commented out for brevity)
                # sys.stdout.write(f"\n[Tool Result: {event.get('data', {}).get('status')}]\n")
                pass
            
            elif event_type == "error":
                sys.stderr.write(f"\n[Error: {event.get('data', {}).get('message')}]\n")
            
            elif event_type == "result":
                sys.stdout.write("\n[Done]\n")
                
        except json.JSONDecodeError:
            # Not JSON, maybe raw output, just pass it through
            sys.stdout.write(line)
            sys.stdout.flush()

if __name__ == "__main__":
    prettify()
