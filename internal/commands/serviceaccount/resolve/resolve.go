package resolve

import (
	"fmt"
	"strconv"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

func ServiceAccountID(client *gitlab.Client, group string, serviceAccount string) (int64, error) {
	if id, err := strconv.ParseInt(serviceAccount, 10, 64); err == nil {
		return id, nil
	}

	opts := &gitlab.ListServiceAccountsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	accounts, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.GroupServiceAccount, *gitlab.Response, error) {
		return client.Groups.ListServiceAccounts(group, opts, p)
	})
	if err != nil {
		return 0, err
	}

	var matches []*gitlab.GroupServiceAccount
	for _, a := range accounts {
		if a.Name == serviceAccount || a.UserName == serviceAccount {
			matches = append(matches, a)
		}
	}

	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("no service account found with the name %q in group %q", serviceAccount, group)
	case 1:
		return matches[0].ID, nil
	default:
		return 0, fmt.Errorf("multiple service accounts found with the name %q in group %q; use the numeric ID instead", serviceAccount, group)
	}
}
