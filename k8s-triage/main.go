package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/google/go-github/v40/github"
	"golang.org/x/oauth2"
)

var access_token = ""

func main() {

	access_token = os.Getenv("ACCESS_TOKEN")
	access_token = strings.Trim(access_token, "\n\t ")

	if len(access_token) == 0 {
		fmt.Printf("access_token is needed")
		os.Exit(2)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting the web server on port %v, access token: %v***\n", port, access_token[0:5])

	http.HandleFunc("/", landing)
	http.HandleFunc("/triage", landing)

	http.HandleFunc("/triage/node-prs", nodePRsIndex)
	http.HandleFunc("/triage/node-prs/addIssuesFor43", nodePRsAddIssuesFor43)
	http.HandleFunc("/triage/node-prs/addIssuesFor49", nodePRsAddIssuesFor49)
	http.HandleFunc("/triage/node-prs/addIssuesFor59", nodePRsAddIssuesFor59)
	http.HandleFunc("/triage/node-prs/waitingOnAuthorFor43", nodePRsWaitingOnAuthorFor43)
	http.HandleFunc("/triage/node-prs/waitingOnAuthorFor49", nodePRsWaitingOnAuthorFor49)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func landing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Wrong page %s", r.URL.Path[1:])
}

func nodePRsIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
		<h1>Triage dashboard</h1>
		<h2>SIG Node CI/Test Board</h2>
    <a href="https://github.com/orgs/kubernetes/projects/43">Project board</a>
		<button onclick="processIssues('/triage/node-prs/addIssuesFor43')">Add Issues</button>
		<button onclick="processIssues('/triage/node-prs/waitingOnAuthorFor43')">Process Waiting On Author</button>
		<h2>SIG Node PR Triage</h2>
    <a href="https://github.com/orgs/kubernetes/projects/49">Project board</a>
		<button onclick="processIssues('/triage/node-prs/addIssuesFor49')">Add Issues</button>
		<button onclick="processIssues('/triage/node-prs/waitingOnAuthorFor49')">Process Waiting On Author</button>
		<h2>SIG Node Bugs</h2>
    <a href="https://github.com/orgs/kubernetes/projects/59">Project board</a>
		<button onclick="processIssues('/triage/node-prs/addIssuesFor59')">Add Issues</button>

		<h2>Output:</h2>
		<div id="result"></div>
		<script>
			function processIssues(query) {
				fetch(query).then(response => response.text()).then(result => {
					document.getElementById('result').innerHTML = result;
				});
			}
		</script>
	`)
}

func getClient(ctx context.Context) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: access_token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	return client
}

func getCardContentDetails(card *github.ProjectCard) (org string, repo string, id int, isIssue bool, err error) {
	if card.GetContentURL() == "" {
		return "", "", 0, false, errors.New("no content url")
	}
	u, err := url.Parse(card.GetContentURL())
	if err != nil {
		return "", "", 0, false, err
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) < 6 {
		return "", "", 0, false, errors.New("not enough parts")
	}

	org = parts[2]
	repo = parts[3]
	id, err = strconv.Atoi(parts[5])
	if err != nil {
		return "", "", 0, false, err
	}
	isIssue = parts[4] == "issues"
	return org, repo, id, isIssue, nil
}

func writeCards(ctx context.Context, sb *strings.Builder, cards []*github.ProjectCard) error {
	for _, card := range cards {
		if card.GetContentURL() == "" {
			continue
		}
		org, repo, id, isIssue, err := getCardContentDetails(card)
		if err != nil {
			return err
		}
		if isIssue {
			sb.WriteString(fmt.Sprintf("card: %v, issue: https://github.com/%v/%v/issue/%d\n", card.GetID(), org, repo, id))
		} else {
			sb.WriteString(fmt.Sprintf("card: %v, pr: https://github.com/%v/%v/pull/%d\n", card.GetID(), org, repo, id))
		}
	}
	return nil
}

func returnError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "Error: %v", err)
}

func processAddIssuesToColumn(ctx context.Context, w http.ResponseWriter, r *http.Request, org string, projectId int, columnName string, queries []string) {
	client := getClient(ctx)

	column, err := getColumn(ctx, client, org, projectId, columnName)
	if err != nil {
		returnError(w, errors.Wrap(err, "cannot find a column"))
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Issues for project https://github.com/orgs/%v/projects/%d (column %q)\n", org, projectId, columnName))

	for _, query := range queries {
		cards, err := addIssuesToColumn(ctx, client, query, column.GetID())
		if err != nil {
			returnError(w, errors.Wrapf(err, "cannot add issues to column for query %q", query))
			return
		}
		err = writeCards(ctx, &sb, cards)
		if err != nil {
			returnError(w, errors.Wrapf(err, "cannot write cards to output for query %q", query))
			return
		}
	}

	fmt.Fprintf(w, sb.String())
}

func nodePRsAddIssuesFor43(w http.ResponseWriter, r *http.Request) {
	org := "kubernetes"
	projectId := 43
	columnName := "Triage"
	queries := []string{
		"is:pr is:open label:sig/node -project:kubernetes/43 repo:kubernetes/test-infra",
		"is:open label:sig/node+-project:kubernetes/43+repo:kubernetes/test-infra",
		"is:open label:sig/node is:pr label:area/test -project:kubernetes/43 repo:kubernetes/kubernetes",
		"is:issue is:open label:sig/node  label:area/test -project:kubernetes/43 repo:kubernetes/kubernetes",
		"is:open label:sig/node is:pr label:kind/failing-test -project:kubernetes/43 repo:kubernetes/kubernetes",
		"is:issue is:open label:sig/node label:kind/failing-test -project:kubernetes/43 repo:kubernetes/kubernetes",
	}

	fmt.Printf("Processing node PRs for project %v", projectId)

  ctx := context.Background()

	processAddIssuesToColumn(ctx, w, r, org, projectId, columnName, queries)
}

func nodePRsAddIssuesFor59(w http.ResponseWriter, r *http.Request) {
	org := "kubernetes"
	projectId := 59
	columnName := "Triage"
	queries := []string{
		"is:open label:sig/node is:issue label:kind/bug org:kubernetes -project:kubernetes/59",
	}

	fmt.Printf("Processing node PRs for project %v", projectId)

  ctx := context.Background()

	processAddIssuesToColumn(ctx, w, r, org, projectId, columnName, queries)
}

func nodePRsAddIssuesFor49(w http.ResponseWriter, r *http.Request) {
	org := "kubernetes"
	projectId := 49
	columnName := "Triage"
	queries := []string{
		"is:open label:sig/node is:pr org:kubernetes -project:kubernetes/49",
	}

	fmt.Printf("Processing node PRs for project %v", projectId)

  ctx := context.Background()

	processAddIssuesToColumn(ctx, w, r, org, projectId, columnName, queries)
}

func getColumn(ctx context.Context, client *github.Client, org string, projectNumber int, columnsName string) (*github.ProjectColumn, error) {
	projects, _, err := client.Organizations.ListProjects(ctx, org, &github.ProjectListOptions{State: "open", ListOptions: github.ListOptions{Page:1, PerPage: 100} })

	if err != nil {
		fmt.Printf("Organizations.ListProjects returned error: %v", err)
		return nil, fmt.Errorf("Organizations.ListProjects returned error: %w", err)
	}

	var targetProject *github.Project

	for _, p := range projects {
		//fmt.Printf("Project: %d %s %s %d\n", *p.ID, *p.Name, *p.HTMLURL, *p.Number)
		if *p.Number == projectNumber {
			targetProject = p
			break
		}
	}

	if targetProject == nil {
		fmt.Printf("Project not found")
		return nil, errors.New("Project not found")
	}

	columns, _, err := client.Projects.ListProjectColumns(ctx, *targetProject.ID, &github.ListOptions{Page:1, PerPage: 100})

	if err != nil {
		fmt.Printf("Projects.ListProjectColumns returned error: %v", err)
		return nil, fmt.Errorf("Projects.ListProjectColumns returned error: %w", err)
	}

	fmt.Printf("Project: %s\n", *targetProject.URL)


	var targetColumn *github.ProjectColumn
	for _, c := range columns {
		//fmt.Printf("Column: %d %s\n", *c.ID, *c.Name)

		if *c.Name == columnsName {
			targetColumn = c
		}
	}

	if targetColumn == nil {
		fmt.Printf("Column not found")
		return nil, errors.New("Column not found")
	}

	fmt.Printf("Column: %d %s\n", *targetColumn.ID, *targetColumn.Name)

	return targetColumn, nil

}

func addIssuesToColumn(ctx context.Context, client *github.Client, query string, columnID int64) ([]*github.ProjectCard, error) {
	result := []*github.ProjectCard{}

	opts := &github.SearchOptions {
		Sort: "forks",
		Order: "desc",
		ListOptions: github.ListOptions{Page: 1, PerPage: 100},
	}

	issues, resp, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search for issues (response: %q)", resp)
	}

	for _, issue := range issues.Issues {
		//fmt.Printf("Issue: %d %s %s %d\n", *issue.ID, *issue.NodeID, *issue.Title, *issue.Number)

		input := &github.ProjectCardOptions{
			ContentID:   *issue.ID,
			ContentType: "Issue",
		}

		card, resp, err := client.Projects.CreateProjectCard(ctx, columnID, input)

		if err != nil {
			fmt.Printf("Projects.CreateProjectCard returned error: %v, %q", err, resp)
			return nil, errors.Wrapf(err, "failed to create a project card (response: %q)", resp)
		}

		// move new card to the bottom, ignoring errors
		_, err = client.Projects.MoveProjectCard(ctx, card.GetID(), &github.ProjectCardMoveOptions{Position: "bottom", ColumnID: columnID})

		result = append(result, card)
	}

	return result, nil
}

func processWaitingOnAuthor(ctx context.Context, w http.ResponseWriter, r *http.Request, client *github.Client, org string, projectId int, targetColumnName string, excludedColumnNames []string) {
	targetColumn, err := getColumn(ctx, client, org, projectId, targetColumnName)

	if err != nil {
		returnError(w, errors.Wrapf(err, "cannot get column ID for %v", targetColumnName))
		return
	}

	excludedColumns := map[int64]struct{}{targetColumn.GetID(): {}}

	for _, columnName := range excludedColumnNames {
		column, err := getColumn(ctx, client, org, projectId, columnName)
		if err != nil {
			returnError(w, errors.Wrapf(err, "cannot get column ID for %v", columnName))
			return
		}
		excludedColumns[column.GetID()] = struct{}{}
	}

	projects, _, err := client.Organizations.ListProjects(ctx, org, &github.ProjectListOptions{State: "open", ListOptions: github.ListOptions{Page:1, PerPage: 100} })

	if err != nil {
		returnError(w, errors.Wrapf(err, "error listing projects for %v", org))
		return
	}

	var targetProject *github.Project

	for _, p := range projects {
		if *p.Number == projectId {
			targetProject = p
			break
		}
	}

	if targetProject == nil {
		returnError(w, errors.New("project not found"))
		return
	}

	columns, _, err := client.Projects.ListProjectColumns(ctx, *targetProject.ID, &github.ListOptions{Page:1, PerPage: 100})

	if err != nil {
		returnError(w, errors.Wrapf(err, "error listing columns for project %v", *targetProject.URL))
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Moving needs-rebase issues for project https://github.com/orgs/%v/projects/%d\n", org, projectId))

	result := []*github.ProjectCard{}

	for _, column := range columns {
		if _, ok := excludedColumns[column.GetID()]; ok {
			continue
		}

		not_archived := "not_archived"
		opts := &github.ProjectCardListOptions{
			ArchivedState: &not_archived,
			ListOptions: github.ListOptions{Page: 1, PerPage: 500},
		}

		cards, _, err := client.Projects.ListProjectCards(ctx, column.GetID(), opts)

		if err != nil {
			returnError(w, errors.Wrapf(err, "error listing cards for column %v", column.GetURL()))
			return
		}

		for _, card := range cards {
			u, err := url.Parse(card.GetContentURL())
			if err != nil {
				fmt.Println(err)
				continue
			}

			// See https://docs.github.com/en/rest/projects/cards?apiVersion=2022-11-28#get-a-project-card
			// "content_url": "https://api.github.com/repos/api-playground/projects-test/issues/3",
			parts := strings.Split(u.Path, "/")

			if len(parts) < 5 {
				// this is not an issue or pull request
				continue
			}

			labels := []*github.Label{}

			org, repo, id, isIssue, err := getCardContentDetails(card)

			if err != nil {
				fmt.Println(err)
				continue
			}

			if isIssue {
				issue, _, err := client.Issues.Get(ctx, org, repo, id)
				if err != nil {
					fmt.Println(err)
					continue
				}

				labels = issue.Labels
			} else {
				pr, _, err := client.PullRequests.Get(ctx, org, repo, id)
				if err != nil {
					fmt.Println(err)
					continue
				}

				labels = pr.Labels
			}

			for _, label := range labels {
				// query all cards from the project with the label "needs-rebase" and move them to the "PRs Waiting on Author" column

				if label.GetName() == "needs-rebase" {
					fmt.Printf("Found needs-rebase card: %d %s\n", card.GetID(), card.GetContentURL())

					result = append(result, card)

					client.Projects.MoveProjectCard(ctx, card.GetID(), &github.ProjectCardMoveOptions{Position: "bottom", ColumnID: targetColumn.GetID()})
				}
			}
		}
	}

	err = writeCards(ctx, &sb, result)
	if err != nil {
		returnError(w, errors.Wrap(err, "error writing cards"))
		return
	}

	fmt.Fprintf(w, sb.String())
}

func nodePRsWaitingOnAuthorFor43(w http.ResponseWriter, r *http.Request) {
  ctx := context.Background()
	client := getClient(ctx)

	org := "kubernetes"
	projectId := 43
	targetColumnName := "PRs Waiting on Author"
	excludedColumnNames := []string{"Done", "Archive-it"}

	processWaitingOnAuthor(ctx, w, r, client, org, projectId, targetColumnName, excludedColumnNames)
}

func nodePRsWaitingOnAuthorFor49(w http.ResponseWriter, r *http.Request) {
  ctx := context.Background()
	client := getClient(ctx)

	org := "kubernetes"
	projectId := 49
	targetColumnName := "Waiting on Author"
	excludedColumnNames := []string{"Done"}

	processWaitingOnAuthor(ctx, w, r, client, org, projectId, targetColumnName, excludedColumnNames)
}

