---
title: Agent Skills Example
stage: Create
group: Code Review
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Agent Skills Example

This page provides examples of how to structure and use Agent Skills with the GitLab CLI MCP server.

## Directory Structure

Agent Skills are organized in directories with the following structure:

```plaintext
skills/
├── my-skill/
│   ├── SKILL.md           # Required: Skill definition with YAML frontmatter
│   ├── references/        # Optional: Reference documents
│   │   ├── api-docs.md
│   │   └── examples.json
│   ├── scripts/           # Optional: Helper scripts
│   │   └── setup.sh
│   └── assets/            # Optional: Images and other assets
│       └── diagram.png
└── another-skill/
    └── SKILL.md
```

## Example SKILL.md

Here's a complete example of a SKILL.md file with all available fields:

```markdown
---
name: gitlab-api-helper
description: Helps interact with GitLab API endpoints for common operations
license: MIT
compatibility: ">=1.0.0"
metadata:
  author: GitLab Team
  version: 1.2.0
  category: api
allowed-tools:
  - glab_api
  - glab_project_view
---

# GitLab API Helper

This skill provides guidance for working with GitLab API endpoints.

## Overview

Use this skill to:
- Understand GitLab API authentication
- Make common API requests
- Handle pagination and rate limiting

## Authentication

GitLab API supports several authentication methods:
1. Personal Access Tokens
2. OAuth2 tokens
3. Job tokens (in CI/CD)

## Common Endpoints

### Projects API
- GET /api/v4/projects/:id
- GET /api/v4/projects/:id/issues

### Issues API
- POST /api/v4/projects/:id/issues
- PUT /api/v4/projects/:id/issues/:issue_iid

## Best Practices

1. Always use pagination for large result sets
2. Respect rate limits (300 requests per minute for authenticated users)
3. Use specific scopes for personal access tokens

## See Also

Check the references/ directory for:
- Complete API documentation
- Example request/response payloads
```

## Minimal Example

A minimal SKILL.md only requires name and description:

```markdown
---
name: simple-skill
description: A simple skill with minimal configuration
---

# Simple Skill

This is a basic skill with only the required fields.

You can include any markdown content here to provide
instructions and guidance.
```

## Using with MCP Server

To expose your skills through the MCP server:

```console
$ glab mcp serve --read-skill /path/to/skills/directory
```

Once the server is running, AI assistants can discover and use your skills through three progressive disclosure tools:

### 1. List Available Skills

```plaintext
Tool: skill_list_metadata
Returns: JSON array of all skills with name and description
```

This provides a quick overview (~100 tokens per skill) to help the AI decide which skills are relevant.

### 2. Read Skill Instructions

```plaintext
Tool: skill_read_markdown_{skillname}
Returns: Full SKILL.md content with frontmatter and body
```

This provides detailed instructions (<5000 tokens typically) for using the skill.

### 3. Access Reference Materials

```plaintext
Tool: skill_read_reference_{skillname}
Parameters: filename (path within references/ directory)
Returns: Content of the specified reference file
```

This provides additional resources as needed, keeping token usage efficient.

## Progressive Disclosure

The three-tier approach follows the Agent Skills specification:

1. **Metadata** (~100 tokens): Quick overview to determine relevance
2. **Markdown** (<5000 tokens): Detailed instructions and guidance
3. **References** (as needed): Additional documentation, examples, schemas

This pattern allows AI assistants to efficiently discover and use skills without loading unnecessary content.

## Validation Rules

Skill names must:
- Be 1-64 characters long
- Use lowercase alphanumeric characters and hyphens only
- Not start or end with hyphens
- Not contain consecutive hyphens

Skill descriptions must:
- Be 1-1024 characters long
- Provide a clear, concise summary of the skill's purpose

## Example Use Case

An AI assistant helping a user with GitLab API integration might:

1. Call `skill_list_metadata` to see available skills
2. Find "gitlab-api-helper" in the results
3. Call `skill_read_markdown_gitlab-api-helper` to get detailed instructions
4. Call `skill_read_reference_gitlab-api-helper` with `filename: "api-docs.md"` for specific API documentation

This progressive approach minimizes token usage while providing comprehensive guidance.
