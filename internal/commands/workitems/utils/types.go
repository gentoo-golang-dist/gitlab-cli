package utils

import (
	"fmt"
	"strings"
)

// IssueType enum values from GitLab GraphQL API
// Used in workItems(types: [IssueType!]) filter parameter
// NOTE: These types will become customizable in future GitLab releases.
// This list is for reference only - API is source of truth for validation.
const (
	// Group-level work items
	TypeEpic      = "EPIC"
	TypeObjective = "OBJECTIVE"
	TypeKeyResult = "KEY_RESULT"
	// Project-level work items
	TypeIssue       = "ISSUE"
	TypeTask        = "TASK"
	TypeIncident    = "INCIDENT"
	TypeTicket      = "TICKET"
	TypeRequirement = "REQUIREMENT"
	TypeTestCase    = "TEST_CASE"
)

// AllKnownTypes contains current default work item types for reference.
// NOTE: This list will become outdated when work item types become customizable.
// Do not use for validation - API is the source of truth.
var AllKnownTypes = []string{
	TypeEpic,
	TypeObjective,
	TypeKeyResult,
	TypeIssue,
	TypeTask,
	TypeIncident,
	TypeTicket,
	TypeRequirement,
	TypeTestCase,
}

// ValidateTypes performs minimal format validation on work item types.
// Only checks that types are non-empty and non-whitespace.
// The GraphQL API is responsible for validating actual type names.
func ValidateTypes(types []string) error {
	for _, t := range types {
		if strings.TrimSpace(t) == "" {
			return fmt.Errorf("empty work item type not allowed")
		}
	}
	return nil
}
