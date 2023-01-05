package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

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
	http.HandleFunc("/triage/node-prs/addIssues", nodePRsAddIssues)
	http.HandleFunc("/triage/node-prs/needsRebase", nodePRsNeedsRebase)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func landing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Wrong page %s", r.URL.Path[1:])
}

func nodePRsIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
		<h1>Hello, World!</h1>
		<button onclick="addIssues()">Add Issues</button>
		<button onclick="processNeedsRebase()">Process Needs Rebase</button>
		<div id="result"></div>
		<script>
			function addIssues() {
				fetch('/triage/node-prs/addIssues').then(response => response.text()).then(result => {
					document.getElementById('result').innerHTML = result;
				});
			}
			function processNeedsRebase() {
				fetch('/triage/node-prs/needsRebase').then(response => response.text()).then(result => {
					document.getElementById('result').innerHTML = result;
				});
			}
		</script>
	`)
}

func nodePRsAddIssues(w http.ResponseWriter, r *http.Request) {

	fmt.Printf("Processing node PRs")

  ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: access_token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)


	columnID, err := getColumnID(ctx, client, "kubernetes", 43, "Triage")

	if err != nil {
		fmt.Fprintf(w, "something went wrong: %q", err)
		return
	}

	addIssuesToColumn(ctx, client, "is:pr is:open label:sig/node -project:kubernetes/43 repo:kubernetes/test-infra", columnID)
	addIssuesToColumn(ctx, client, "is:open label:sig/node+-project:kubernetes/43+repo:kubernetes/test-infra", columnID)
	addIssuesToColumn(ctx, client, "is:open label:sig/node is:pr label:area/test -project:kubernetes/43 repo:kubernetes/kubernetes", columnID)
	addIssuesToColumn(ctx, client, "is:issue is:open label:sig/node  label:area/test -project:kubernetes/43 repo:kubernetes/kubernetes", columnID)
	addIssuesToColumn(ctx, client, "is:open label:sig/node is:pr label:kind/failing-test -project:kubernetes/43 repo:kubernetes/kubernetes", columnID)
	addIssuesToColumn(ctx, client, "is:issue is:open label:sig/node label:kind/failing-test -project:kubernetes/43 repo:kubernetes/kubernetes", columnID)

	columnID, err = getColumnID(ctx, client, "kubernetes", 59, "Triage")

	if err != nil {
		fmt.Fprintf(w, "something wrong: %q", err)
		return
	}

	addIssuesToColumn(ctx, client, "is:open label:sig/node is:issue label:kind/bug org:kubernetes -project:kubernetes/59", columnID)

	columnID, err = getColumnID(ctx, client, "kubernetes", 49, "Triage")

	if err != nil {
		fmt.Fprintf(w, "something wrong: %q", err)
		return
	}

	addIssuesToColumn(ctx, client, "is:open label:sig/node is:pr org:kubernetes -project:kubernetes/49", columnID)

	fmt.Fprintf(w, "Hello, World!\n")
}

func getColumnID(ctx context.Context, client *github.Client, org string, projectNumber int, columnsName string) (int64, error) {
	projects, _, err := client.Organizations.ListProjects(ctx, org, &github.ProjectListOptions{State: "open", ListOptions: github.ListOptions{Page:1, PerPage: 100} })

	if err != nil {
		fmt.Printf("Organizations.ListProjects returned error: %v", err)
		return -1, fmt.Errorf("Organizations.ListProjects returned error: %w", err)
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
		return -1, errors.New("Project not found")
	}

	columns, _, err := client.Projects.ListProjectColumns(ctx, *targetProject.ID, &github.ListOptions{Page:1, PerPage: 100})

	if err != nil {
		fmt.Printf("Projects.ListProjectColumns returned error: %v", err)
		return -1, fmt.Errorf("Projects.ListProjectColumns returned error: %w", err)
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
		return -1, errors.New("Column not found")
	}

	fmt.Printf("Column: %d %s\n", *targetColumn.ID, *targetColumn.Name)

	return *targetColumn.ID, nil

}

func addIssuesToColumn(ctx context.Context, client *github.Client, query string, columnID int64) error {
	opts := &github.SearchOptions {
		Sort: "forks",
		Order: "desc",
		ListOptions: github.ListOptions{Page: 1, PerPage: 100},
	}

	result, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		fmt.Printf("Search.Issues returned error: %v", err)
		return err
	}

	for _, issue := range result.Issues {
		fmt.Printf("Issue: %d %s %s %d\n", *issue.ID, *issue.NodeID, *issue.Title, *issue.Number)


		if err != nil {
			fmt.Printf("Organizations.ListProjects returned error: %v", err)
			return err
		}

		input := &github.ProjectCardOptions{
			ContentID:   *issue.ID,
			ContentType: "Issue",
		}

		card, resp, err := client.Projects.CreateProjectCard(ctx, columnID, input)

		// move new card to the bottom
		_, err = client.Projects.MoveProjectCard(ctx, card.GetID(), &github.ProjectCardMoveOptions{Position: "bottom"})

		if err != nil {
			fmt.Printf("Projects.CreateProjectCard returned error: %v, %q", err, resp)
			return err
		}

		fmt.Printf("Card: %s\n", *card.URL)
	}

	return nil

}

func nodePRsNeedsRebase(w http.ResponseWriter, r *http.Request) {

  ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: access_token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)


	targetColumnID, err := getColumnID(ctx, client, "kubernetes", 43, "PRs Waiting on Author")
	doneColumnID, err := getColumnID(ctx, client, "kubernetes", 43, "Done")
	archiveItColumnID, err := getColumnID(ctx, client, "kubernetes", 43, "Archive-it")

	if err != nil {
		fmt.Fprintf(w, "something went wrong: %q", err)
		return
	}



	projects, _, err := client.Organizations.ListProjects(ctx, "kubernetes", &github.ProjectListOptions{State: "open", ListOptions: github.ListOptions{Page:1, PerPage: 100} })

	if err != nil {
		fmt.Printf("Organizations.ListProjects returned error: %v", err)
		return //-1, fmt.Errorf("Organizations.ListProjects returned error: %w", err)
	}

	var targetProject *github.Project

	for _, p := range projects {
		//fmt.Printf("Project: %d %s %s %d\n", *p.ID, *p.Name, *p.HTMLURL, *p.Number)
		if *p.Number == 43 {
			targetProject = p
			break
		}
	}

	if targetProject == nil {
		fmt.Printf("Project not found")
		return //-1, errors.New("Project not found")
	}


	columns, _, err := client.Projects.ListProjectColumns(ctx, *targetProject.ID, &github.ListOptions{Page:1, PerPage: 100})

	if err != nil {
		fmt.Printf("Projects.ListProjectColumns returned error: %v", err)
		return
	}

	for _, column := range columns {
		//fmt.Printf("Column: %d %s\n", *column.ID, *column.Name)

		if column.GetID() == targetColumnID || column.GetID() == doneColumnID || column.GetID() == archiveItColumnID {
			continue
		}

		not_archived := "not_archived"
		opts := &github.ProjectCardListOptions{
			ArchivedState: &not_archived,
			ListOptions: github.ListOptions{Page: 1, PerPage: 500},
		}

		cards, _, err := client.Projects.ListProjectCards(ctx, *column.ID, opts)

		if err != nil {
			fmt.Printf("Projects.ListProjectCards returned error: %v", err)
			return
		}

		for _, card := range cards {
			//fmt.Printf("Card: %d %s\n", card.GetID(), card.GetContentURL())

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

			// Check if the card is an issue or pull request
			if parts[4] == "issues" {
				issueID, err := strconv.Atoi(parts[5])
				if err != nil {
					fmt.Println(err)
					continue
				}

				issue, _, err := client.Issues.Get(ctx, parts[2], parts[3], issueID)
				if err != nil {
					fmt.Println(err)
					continue
				}

				labels = issue.Labels
			} else if parts[4] == "pulls" {
				prID, err := strconv.Atoi(parts[5])
				if err != nil {
					fmt.Println(err)
					continue
				}

				pr, _, err := client.PullRequests.Get(ctx, parts[2], parts[3], prID)
				if err != nil {
					fmt.Println(err)
					continue
				}

				labels = pr.Labels
			}


			for _, label := range labels {
				//fmt.Println(label.GetName())

				// query all cards from the project with the label "needs-rebase" and move them to the "PRs Waiting on Author" column

				if label.GetName() == "needs-rebase" {
					fmt.Printf("Found needs-rebase card: %d %s\n", card.GetID(), card.GetContentURL())

					client.Projects.MoveProjectCard(ctx, card.GetID(), &github.ProjectCardMoveOptions{Position: "bottom", ColumnID: targetColumnID})

				}
			}
		}
	}
}




