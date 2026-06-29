# Remote Code Router PRD

## Goal

Let a CPA server operator expose one stable client model name while switching the real upstream model from the plugin page.

## Requirements

- Register virtual models.
- Import `/v1/models` from the plugin page.
- Store imported candidates in plugin state.
- Switch the active candidate from the plugin page.
- Execute the selected candidate through CPA host model callbacks.
