package api

import (
	"sort"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// GetMR returns an MR
// Attention: this is a global variable and may be overridden in tests.
var GetMR = func(client *gitlab.Client, projectID any, mrID int64, opts *gitlab.GetMergeRequestsOptions) (*gitlab.MergeRequest, error) {
	mr, _, err := client.MergeRequests.GetMergeRequest(projectID, mrID, opts)
	if err != nil {
		return nil, err
	}

	return mr, nil
}

// ListGroupMRs retrieves merge requests for a given group with optional filtering by assignees or reviewers.
//
// Parameters:
//   - client: A GitLab client instance.
//   - groupID: The ID or name of the group.
//   - opts: GitLab-specific options for listing group merge requests.
//   - listOpts: Optional list of arguments to filter by assignees or reviewers.
//     May be any combination of api.WithMRAssignees and api.WithMRReviewers.
//
// Returns:
//   - A slice of GitLab merge request objects and an error, if any.
//
// Example usage:
//
//	groupMRs, err := api.ListGroupMRs(client, "my-group", &gitlab.ListGroupMergeRequestsOptions{},
//		api.WithMRAssignees([]int{123}),
//		api.WithMRReviewers([]int{456, 789}))
func ListGroupMRs(client *gitlab.Client, projectID any, opts *gitlab.ListGroupMergeRequestsOptions, listOpts ...CliListMROption) ([]*gitlab.BasicMergeRequest, error) {
	composedListOpts := composeCliListMROptions(listOpts...)
	assigneeIds, reviewerIds := composedListOpts.assigneeIds, composedListOpts.reviewerIds

	if len(assigneeIds) > 0 || len(reviewerIds) > 0 {
		return listGroupMRsWithAssigneesOrReviewers(client, projectID, opts, assigneeIds, reviewerIds)
	} else {
		return listGroupMRsBase(client, projectID, opts)
	}
}

func listGroupMRsBase(client *gitlab.Client, groupID any, opts *gitlab.ListGroupMergeRequestsOptions) ([]*gitlab.BasicMergeRequest, error) {
	if opts.PerPage == 0 {
		opts.PerPage = DefaultListLimit
	}

	mrs, _, err := client.MergeRequests.ListGroupMergeRequests(groupID, opts)
	if err != nil {
		return nil, err
	}
	return mrs, nil
}

func listGroupMRsWithAssigneesOrReviewers(client *gitlab.Client, groupID any, opts *gitlab.ListGroupMergeRequestsOptions, assigneeIds []int, reviewerIds []int) ([]*gitlab.BasicMergeRequest, error) {
	if opts.PerPage == 0 {
		opts.PerPage = DefaultListLimit
	}
	return fanOutMRListByAssigneeReviewer(
		assigneeIds, reviewerIds,
		func(v *gitlab.AssigneeIDValue) { opts.AssigneeID = v },
		func(v *gitlab.ReviewerIDValue) { opts.ReviewerID = v },
		opts.OrderBy,
		func() ([]*gitlab.BasicMergeRequest, error) { return listGroupMRsBase(client, groupID, opts) },
	)
}

// ListMRs retrieves merge requests for a given project with optional filtering by assignees or reviewers.
//
// Parameters:
//   - client: A GitLab client instance.
//   - projectID: The ID or name of the project.
//   - opts: GitLab-specific options for listing merge requests.
//   - listOpts: Optional list of arguments to filter by assignees or reviewers.
//     May be any combination of api.WithMRAssignees and api.WithMRReviewers.
//
// Returns:
//   - A slice of GitLab merge request objects and an error, if any.
//
// Example usage:
//
//	mrs, err := api.ListMRs(client, "my-group", &gitlab.ListProjectMergeRequestsOptions{},
//		api.WithMRAssignees([]int{123, 456}),
//		api.WithMRReviewers([]int{789}))
//
// Attention: this is a global variable and may be overridden in tests.
var ListMRs = func(client *gitlab.Client, projectID any, opts *gitlab.ListProjectMergeRequestsOptions, listOpts ...CliListMROption) ([]*gitlab.BasicMergeRequest, error) {
	composedListOpts := composeCliListMROptions(listOpts...)
	assigneeIds, reviewerIds := composedListOpts.assigneeIds, composedListOpts.reviewerIds

	if len(assigneeIds) > 0 || len(reviewerIds) > 0 {
		return listMRsWithAssigneesOrReviewers(client, projectID, opts, assigneeIds, reviewerIds)
	} else {
		return listMRsBase(client, projectID, opts)
	}
}

func listMRsBase(client *gitlab.Client, projectID any, opts *gitlab.ListProjectMergeRequestsOptions) ([]*gitlab.BasicMergeRequest, error) {
	if opts.PerPage == 0 {
		opts.PerPage = DefaultListLimit
	}

	mrs, _, err := client.MergeRequests.ListProjectMergeRequests(projectID, opts)
	if err != nil {
		return nil, err
	}
	return mrs, nil
}

func listMRsWithAssigneesOrReviewers(client *gitlab.Client, projectID any, opts *gitlab.ListProjectMergeRequestsOptions, assigneeIds []int, reviewerIds []int) ([]*gitlab.BasicMergeRequest, error) {
	if opts.PerPage == 0 {
		opts.PerPage = DefaultListLimit
	}
	return fanOutMRListByAssigneeReviewer(
		assigneeIds, reviewerIds,
		func(v *gitlab.AssigneeIDValue) { opts.AssigneeID = v },
		func(v *gitlab.ReviewerIDValue) { opts.ReviewerID = v },
		opts.OrderBy,
		func() ([]*gitlab.BasicMergeRequest, error) { return listMRsBase(client, projectID, opts) },
	)
}

// ListAllMRs queries the user-level /merge_requests endpoint (no
// project or group scope), with optional assignee / reviewer
// fan-out matching the project- and group-scoped helpers. Used
// when no -R or --group is in scope, notably MCP standalone calls
// where the caller wants everything they can see.
//
// Attention: this is a global variable and may be overridden in tests.
var ListAllMRs = func(client *gitlab.Client, opts *gitlab.ListMergeRequestsOptions, listOpts ...CliListMROption) ([]*gitlab.BasicMergeRequest, error) {
	composedListOpts := composeCliListMROptions(listOpts...)
	assigneeIds, reviewerIds := composedListOpts.assigneeIds, composedListOpts.reviewerIds

	if len(assigneeIds) > 0 || len(reviewerIds) > 0 {
		return listAllMRsWithAssigneesOrReviewers(client, opts, assigneeIds, reviewerIds)
	}
	return listAllMRsBase(client, opts)
}

func listAllMRsBase(client *gitlab.Client, opts *gitlab.ListMergeRequestsOptions) ([]*gitlab.BasicMergeRequest, error) {
	if opts.PerPage == 0 {
		opts.PerPage = DefaultListLimit
	}
	mrs, _, err := client.MergeRequests.ListMergeRequests(opts)
	if err != nil {
		return nil, err
	}
	return mrs, nil
}

func listAllMRsWithAssigneesOrReviewers(client *gitlab.Client, opts *gitlab.ListMergeRequestsOptions, assigneeIds []int, reviewerIds []int) ([]*gitlab.BasicMergeRequest, error) {
	if opts.PerPage == 0 {
		opts.PerPage = DefaultListLimit
	}
	return fanOutMRListByAssigneeReviewer(
		assigneeIds, reviewerIds,
		func(v *gitlab.AssigneeIDValue) { opts.AssigneeID = v },
		func(v *gitlab.ReviewerIDValue) { opts.ReviewerID = v },
		opts.OrderBy,
		func() ([]*gitlab.BasicMergeRequest, error) { return listAllMRsBase(client, opts) },
	)
}

// fanOutMRListByAssigneeReviewer runs runQuery once per assignee
// id, resets the assignee, then once per reviewer id. Results are
// deduped by MR ID and sorted by CreatedAt desc when orderBy is
// nil. Shared by the project, group, and user-level list paths --
// the GitLab API rejects multi-value assignee / reviewer filters
// in a single request, so we fan out and merge here.
func fanOutMRListByAssigneeReviewer(
	assigneeIds, reviewerIds []int,
	setAssigneeID func(*gitlab.AssigneeIDValue),
	setReviewerID func(*gitlab.ReviewerIDValue),
	orderBy *string,
	runQuery func() ([]*gitlab.BasicMergeRequest, error),
) ([]*gitlab.BasicMergeRequest, error) {
	mrMap := make(map[int64]*gitlab.BasicMergeRequest)

	for _, id := range assigneeIds {
		setAssigneeID(gitlab.AssigneeID(id))
		mrs, err := runQuery()
		if err != nil {
			return nil, err
		}
		for _, mr := range mrs {
			mrMap[mr.ID] = mr
		}
	}
	setAssigneeID(nil) // reset because it's Assignee OR Reviewer
	for _, id := range reviewerIds {
		setReviewerID(gitlab.ReviewerID(id))
		mrs, err := runQuery()
		if err != nil {
			return nil, err
		}
		for _, mr := range mrs {
			mrMap[mr.ID] = mr
		}
	}

	out := make([]*gitlab.BasicMergeRequest, 0, len(mrMap))
	for _, mr := range mrMap {
		out = append(out, mr)
	}

	// Sort by CreatedAt when no custom OrderBy is set; otherwise
	// the API's ordering wins. Note this only sorts the current
	// page -- multi-page callers should pass an explicit OrderBy.
	if orderBy == nil {
		sort.Slice(out, func(i, j int) bool {
			return out[i].CreatedAt.After(*out[j].CreatedAt)
		})
	}

	return out, nil
}

// UpdateMR updates an MR
// Attention: this is a global variable and may be overridden in tests.
var UpdateMR = func(client *gitlab.Client, projectID any, mrID int64, opts *gitlab.UpdateMergeRequestOptions) (*gitlab.MergeRequest, error) {
	mr, _, err := client.MergeRequests.UpdateMergeRequest(projectID, mrID, opts)
	if err != nil {
		return nil, err
	}

	return mr, nil
}

// UpdateMR updates an MR
// Attention: this is a global variable and may be overridden in tests.
var DeleteMR = func(client *gitlab.Client, projectID any, mrID int64) error {
	_, err := client.MergeRequests.DeleteMergeRequest(projectID, mrID)
	if err != nil {
		return err
	}

	return nil
}

type cliListMROptions struct {
	assigneeIds []int
	reviewerIds []int
}

type CliListMROption func(*cliListMROptions)

func WithMRAssignees(assigneeIds []int) CliListMROption {
	return func(c *cliListMROptions) {
		c.assigneeIds = assigneeIds
	}
}

func WithMRReviewers(reviewerIds []int) CliListMROption {
	return func(c *cliListMROptions) {
		c.reviewerIds = reviewerIds
	}
}

func composeCliListMROptions(optionSetters ...CliListMROption) *cliListMROptions {
	opts := &cliListMROptions{}
	for _, setter := range optionSetters {
		setter(opts)
	}
	return opts
}
