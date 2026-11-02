package jenkins

// BuildRef 是任务里引用的构建摘要。
type BuildRef struct {
	Number    int    `json:"number"`
	Result    string `json:"result"`
	Timestamp int64  `json:"timestamp"`
	Duration  int64  `json:"duration"`
	Building  bool   `json:"building"`
}

// View 是 Jenkins 的视图（All 或自定义视图）。
type View struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// JobSummary 是任务树里的一项。
type JobSummary struct {
	Name      string    `json:"name"`
	FullName  string    `json:"fullName"`
	URL       string    `json:"url"`
	Color     string    `json:"color"`
	Class     string    `json:"_class"`
	LastBuild *BuildRef `json:"lastBuild"`
	IsFolder  bool      `json:"isFolder"`
	Status    Status    `json:"status"`
	Building  bool      `json:"building"`
}

// ParameterDefinition 是任务参数定义。
type ParameterDefinition struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	DefaultValue string   `json:"defaultValue"`
	Choices      []string `json:"choices"`
}

// BatchTargetInfo 是批量预检结果：确认弹窗用的实时状态与参数定义。
type BatchTargetInfo struct {
	Name       string                `json:"name"`
	FullName   string                `json:"fullName"`
	Status     Status                `json:"status"`
	Building   bool                  `json:"building"`
	Buildable  bool                  `json:"buildable"`
	InQueue    bool                  `json:"inQueue"`
	Parameters []ParameterDefinition `json:"parameters"`
	// Error 只在单个任务读取失败时非空；其余任务照常返回。
	Error string `json:"error"`
}

// JobDetail 是任务详情。
type JobDetail struct {
	Name                string                `json:"name"`
	FullName            string                `json:"fullName"`
	Description         string                `json:"description"`
	URL                 string                `json:"url"`
	Color               string                `json:"color"`
	Class               string                `json:"class"`
	Buildable           bool                  `json:"buildable"`
	InQueue             bool                  `json:"inQueue"`
	LastBuild           *BuildRef             `json:"lastBuild"`
	LastSuccessfulBuild *BuildRef             `json:"lastSuccessfulBuild"`
	LastFailedBuild     *BuildRef             `json:"lastFailedBuild"`
	Parameters          []ParameterDefinition `json:"parameters"`
	Status              Status                `json:"status"`
}

// Cause 是构建触发原因。
type Cause struct {
	ShortDescription string `json:"shortDescription"`
	UserID           string `json:"userId"`
	UserName         string `json:"userName"`
}

// BuildSummary 是构建列表里的一项。
type BuildSummary struct {
	Number      int     `json:"number"`
	Result      string  `json:"result"`
	Building    bool    `json:"building"`
	Timestamp   int64   `json:"timestamp"`
	Duration    int64   `json:"duration"`
	DisplayName string  `json:"displayName"`
	Causes      []Cause `json:"causes"`
	Status      Status  `json:"status"`
	CommitCount int     `json:"commitCount"`
	// EstimatedDuration 是 Jenkins 侧估算的本次构建总耗时，列表用它算进度条。
	EstimatedDuration int64 `json:"estimatedDuration"`
	// Changes 只在内部传给阶段矩阵，不进入 jenkins/builds 的响应体。
	Changes []ChangeSetItem `json:"-"`
}

// Stage 是 Pipeline 构建里的一个阶段。
type Stage struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Status              string `json:"status"`
	StartTimeMillis     int64  `json:"startTimeMillis"`
	DurationMillis      int64  `json:"durationMillis"`
	PauseDurationMillis int64  `json:"pauseDurationMillis"`
}

// StageBuild 是阶段矩阵里的一行构建。
type StageBuild struct {
	Number      int             `json:"number"`
	Result      string          `json:"result"`
	Status      string          `json:"status"`
	Timestamp   int64           `json:"timestamp"`
	Duration    int64           `json:"duration"`
	CommitCount int             `json:"commitCount"`
	Changes     []ChangeSetItem `json:"changes"`
	Stages      []Stage         `json:"stages"`
}

// StageAverage 是某个阶段的平均耗时。
type StageAverage struct {
	Name     string `json:"name"`
	Duration int64  `json:"duration"`
}

// StageMatrix 是阶段视图的完整数据。
type StageMatrix struct {
	Job        string         `json:"job"`
	StageNames []string       `json:"stageNames"`
	Builds     []StageBuild   `json:"builds"`
	Averages   []StageAverage `json:"averages"`
	AverageRun int64          `json:"averageRun"`
}

// ChangeSetItem 是一次提交。
type ChangeSetItem struct {
	CommitID string `json:"commitId"`
	Message  string `json:"message"`
	Author   string `json:"author"`
	// Timestamp 是归一化后的提交时间（epoch 毫秒）：优先 Jenkins timestamp，
	// 缺失时由 date 字符串解析，解析不出来为 0。
	Timestamp int64 `json:"timestamp,omitempty"`
	// FileCount 是本次提交改动的文件数（affectedPaths 长度）。
	FileCount int `json:"fileCount,omitempty"`
	// Body 是提交正文（comment 去掉首行）。只有构建详情返回，阶段矩阵恒为空。
	Body string `json:"body,omitempty"`
}

// Artifact 是构建产物。
type Artifact struct {
	FileName     string `json:"fileName"`
	RelativePath string `json:"relativePath"`
}

// ArtifactInfo 是制品元信息（HEAD 制品 URL 得到）。
type ArtifactInfo struct {
	FileName    string `json:"fileName"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
}

// ArtifactChunk 是制品的一段内容，DataBase64 为原始字节的 base64。
type ArtifactChunk struct {
	Offset     int64  `json:"offset"`
	Length     int    `json:"length"`
	Total      int64  `json:"total"`
	EOF        bool   `json:"eof"`
	DataBase64 string `json:"dataBase64"`
}

// BuildDetail 是单次构建详情。
type BuildDetail struct {
	Number            int             `json:"number"`
	Result            string          `json:"result"`
	Building          bool            `json:"building"`
	Timestamp         int64           `json:"timestamp"`
	Duration          int64           `json:"duration"`
	EstimatedDuration int64           `json:"estimatedDuration"`
	DisplayName       string          `json:"displayName"`
	FullDisplayName   string          `json:"fullDisplayName"`
	URL               string          `json:"url"`
	Causes            []Cause         `json:"causes"`
	ChangeSet         []ChangeSetItem `json:"changeSet"`
	Artifacts         []Artifact      `json:"artifacts"`
	Status            Status          `json:"status"`
}

// ConsoleChunk 是一次增量日志读取的结果。
type ConsoleChunk struct {
	Text      string `json:"text"`
	NextStart int64  `json:"nextStart"`
	MoreData  bool   `json:"moreData"`
}

// QueueItem 是 Jenkins 构建队列里的一项。
type QueueItem struct {
	ID           int    `json:"id"`
	JobName      string `json:"jobName"`
	JobURL       string `json:"jobUrl"`
	Why          string `json:"why"`
	Stuck        bool   `json:"stuck"`
	Blocked      bool   `json:"blocked"`
	BuildableIn  int64  `json:"buildableIn"`
	InQueueSince int64  `json:"inQueueSince"`
	Cause        string `json:"cause"`
}

// Node 是 Jenkins 节点（控制器或代理）。
type Node struct {
	Name               string   `json:"name"`
	Offline            bool     `json:"offline"`
	TemporarilyOffline bool     `json:"temporarilyOffline"`
	NumExecutors       int      `json:"numExecutors"`
	Idle               int      `json:"idle"`
	Busy               int      `json:"busy"`
	Labels             []string `json:"labels"`
	OfflineReason      string   `json:"offlineReason"`
}

// Executor 是节点上的一个执行器槽位；空闲槽位也会返回，供界面显示 0/N 的占位。
type Executor struct {
	// Node 是节点显示名，控制器的 displayName 可能为空串，由前端归一成「内置节点」。
	Node        string `json:"node"`
	NodeOffline bool   `json:"nodeOffline"`
	// Index 是槽位序号，从 1 开始（Jenkins API 不提供）。
	Index int  `json:"index"`
	Idle  bool `json:"idle"`
	// JobName 是任务全名；Queue$Executable 形态（占位中）为空。
	JobName     string `json:"jobName"`
	BuildNumber int    `json:"buildNumber"`
	Building    bool   `json:"building"`
	Timestamp   int64  `json:"timestamp"`
	// EstimatedDuration / Duration 单位毫秒；Duration 在构建期间由服务端实时推进。
	EstimatedDuration int64  `json:"estimatedDuration"`
	Duration          int64  `json:"duration"`
	Result            string `json:"result"`
	// Pending 表示执行器已占位但构建对象还没建好（Queue$Executable）。
	Pending bool `json:"pending"`
}

// Overview 是概览页需要的聚合数据。
type Overview struct {
	Version         string `json:"version"`
	User            string `json:"user"`
	Mode            string `json:"mode"`
	NodeDescription string `json:"nodeDescription"`
	QuietingDown    bool   `json:"quietingDown"`
	QueueLength     int    `json:"queueLength"`
	NodesOnline     int    `json:"nodesOnline"`
	NodesTotal      int    `json:"nodesTotal"`
	ExecutorsBusy   int    `json:"executorsBusy"`
	ExecutorsTotal  int    `json:"executorsTotal"`
}

// StatusFromResult 把 Jenkins 构建结果（SUCCESS 等）归一化为 Status。
func StatusFromResult(result string, building bool) Status {
	if building {
		return StatusBuilding
	}
	switch result {
	case "SUCCESS":
		return StatusSuccess
	case "FAILURE":
		return StatusFailure
	case "UNSTABLE":
		return StatusUnstable
	case "ABORTED":
		return StatusAborted
	case "NOT_BUILT":
		return StatusNotBuilt
	default:
		return StatusUnknown
	}
}
