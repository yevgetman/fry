package epic

type Epic struct {
	Name                 string
	Engine               string
	DockerFromSprint     int
	DockerReadyCmd       string
	DockerReadyTimeout   int
	RequiredTools        []string
	PreflightCmds        []string
	PreSprintCmd         string
	PreIterationCmd      string
	AgentModel           string
	AgentFlags           string
	VerificationFile     string
	MaxHealAttempts      int
	CompactWithAgent     bool
	ReviewBetweenSprints bool
	ReviewEngine         string
	ReviewModel          string
	MaxDeviationScope    int
	Sprints              []Sprint
	TotalSprints         int
}

type Sprint struct {
	Number          int
	Name            string
	MaxIterations   int
	Promise         string
	MaxHealAttempts *int
	Prompt          string
}
