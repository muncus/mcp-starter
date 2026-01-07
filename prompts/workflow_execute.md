---
name: wf-execute
description: implement planned work from an execution plan markdown file
---
# Summary
Read the provided markdown file, containing an engineering plan. Outline the
next steps, and, when confirmed by the user, being implementation of the next
step of the plan.

# Determine next steps

Read the provided plan file completely, and identify the next un-completed work item in the plan.
Determine if any required information is missing, and ask for further guidance from the user.

If you are not sure what to do next or how to proceed, stop immediately.

# Implementation

Implement the next item in the plan, ensuring that the project builds correctly, and passes all tests.

Never commit code that fails to compile, fails tests, or produces other built-time errors.


# Testing
When adding tests, prefer adding a small number of cases rather than exhaustive edge cases. More tests can always be added later.

# Verification

The code must build, and pass tests, as well as any other linting tools provided.

If the requested feature is completed, update the plan document by "checking" the markdown checkbox with an 'x'