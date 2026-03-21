#!/usr/bin/env python3
"""Rebuild the semantic family index."""

from __future__ import annotations

import json

from common import refresh_index


if __name__ == "__main__":
    print(json.dumps(refresh_index(), indent=2))
