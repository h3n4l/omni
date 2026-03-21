# Omni Parser Dashboard Redesign

## Context

The existing dashboard (`scripts/dashboard.go`) is a single-file Go web app that monitors parser engine progress. It works but has UX issues: full innerHTML rebuilds every 3s (breaks hover/scroll), no search/filter, no activity timeline, information overload on a single scrolling page.

## Goals

- **Team collaboration**: shareable, self-explanatory UI
- **Long-term display**: hang on a screen, glanceable without interaction
- **Syntax lookup**: quickly check if a specific statement is supported and to what extent
- **Zero build**: `go run dashboard.go` and open browser

## Tech Stack

Go single-file backend + Preact CDN + HTM (tagged template literals). No node, no build step.

## API Design

| Endpoint | Method | Description |
|----------|--------|-------------|
| `GET /api/status` | GET | All engine progress + BNF data |
| `GET /api/activity` | GET | Recent activity events (ring buffer, last 100) |
| `GET /api/bnf?engine=X&slug=Y` | GET | BNF file content |
| `POST /api/bark?engine=X` | POST | Toggle Bark notification for engine |
| `GET /api/bark` | GET | Current Bark toggle state |
| `GET /` | GET | HTML page |

### Activity Events

Backend polls PROGRESS.json every 3s, diffs against previous snapshot, detects batch status transitions (pending->in_progress, in_progress->done, etc). Events stored in memory ring buffer (100 entries max).

Event structure: `{ timestamp, engine, batch_id, batch_name, old_status, new_status }`

### Bark

Bark key read from `BARK_KEY` environment variable. When empty, Bark functionality is disabled.

## Page Layout

```
+--------------------------------------------------+
|  Header: Omni Parser Dashboard     Notifications  |
+------------+---------------------+---------------+
|            |                     |               |
|  Sidebar   |    Main Panel       |  Activity     |
|  200px     |    flexible         |  Feed 280px   |
|            |                     |               |
|  pg  100%  |  Overview / Engine  |  timeline     |
|  mysql 100%|  detail view        |  entries      |
|  mssql 87% |                     |               |
|  oracle 81%|                     |               |
|            |                     |               |
+------------+---------------------+---------------+
```

- Sidebar: click engine -> show engine detail; click again -> back to overview
- Main default: overview with 4 engine cards side by side
- Min width 1024px, optimal at 1920px

## Component Structure

```
App
+-- Header            title + total progress bar + notification settings
+-- Sidebar           engine list with status icons and percentages
+-- MainPanel
|   +-- OverviewView  default: 4 engine summary cards
|   +-- EngineView    selected engine detail
|       +-- ProgressBar
|       +-- BatchGrid      heatmap with filter buttons (All/Done/Failed/Pending)
|       +-- SearchBar      filters batches and BNF statements simultaneously
|       +-- StatementList  BNF statements, click to expand inline (no modal)
+-- ActivityFeed      right sidebar, newest on top, auto-scroll
```

## Visual Design

- Dark theme: background #0a0e14, cards #161b22
- Semantic colors: green=done, yellow=in_progress, red=failed, purple=auditing
- Batch squares: 32x32 with 6px gap (up from 28x28/4px)
- Tooltips: fixed position below heatmap (not following mouse), no overflow issues
- Activity feed: vertical timeline with colored dots (green=done, yellow=started, red=failed)
- Animations: phase badge pulse, progress bar CSS transition, activity entry fade-in

## State Management

Single top-level Preact state:

```
{
  engines: [],          // from /api/status
  bnf: [],              // from /api/status
  activities: [],       // from /api/activity
  selectedEngine: null, // engine key or null for overview
  searchQuery: '',
  batchFilter: 'all',   // all/done/failed/pending
  barkEnabled: {},
  notifyEnabled: {}     // persisted in localStorage
}
```

- 3s polling interval, /api/status and /api/activity fetched in parallel
- Preact virtual DOM diff handles incremental updates
- Browser notifications: frontend detects phase transitions
- Bark notifications: backend phase watcher

## Removed Features

- Audit trigger button (audits run via driver.sh)
- Cross-engine comparison matrix
- BNF modal (replaced by inline expand)
