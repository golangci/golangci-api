package repoanalyzes

func Start() {
	go launchPendingRepoAnalyzes()
	go restartBrokenRepoAnalyzes()
	go reanalyzeByNewLinters()
	go NewReposLauncher{}.Run()
}
