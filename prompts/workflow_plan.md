---
name: wf-plan
description: create an execution plan for work to be done
---
# Summary
Create a structured implementation plan document for the provided task as a markdown file, in a 'plans' subdirectory of this project.

# Document Structure
The plan document should use the following structure:

## Summary

This section contains a brief high-level overview of the proposed work, including any rationale provided by the user.

## Technical Approach

discuss the intended architecture decisions relevant to this work

## Implementation
A detailed breakdown of individual work items, scoped and sized to be
implemented by an individual developer, in a single Pull Request.

Implementation steps should be structured as a series of un-ordered bullet list
items.

Unless instructed otherwise, include an empty markdown checkbox next to
incompleted items (`[ ]`), and a checked box for those already completed
(`[x]`).

If there are explicit dependencies betweens steps, mention this as a sub-bullet prefixed with 'deps:'

## Testing Strategy

a discussion of how to evalute the success of these changes, including specific
test cases and edge cases where relevant.