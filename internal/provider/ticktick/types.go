package ticktick

type SignOnResponse struct {
	Token    string `json:"token"`
	UserID   string `json:"userId"`
	InboxID  string `json:"inboxId"`
	Username string `json:"username"`
}

type BatchResponse struct {
	InboxID         string         `json:"inboxId"`
	ProjectProfiles []ProjectV2    `json:"projectProfiles"`
	SyncTaskBean    SyncTaskBeanV2 `json:"syncTaskBean"`
}

type SyncTaskBeanV2 struct {
	Update []TaskV2 `json:"update"`
}

type ProjectV2 struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TaskV2 struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"projectId"`
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	Desc          string   `json:"desc"`
	Status        int      `json:"status"`
	Priority      int      `json:"priority"`
	DueDate       string   `json:"dueDate"`
	StartDate     string   `json:"startDate"`
	CompletedTime string   `json:"completedTime"`
	ModifiedTime  string   `json:"modifiedTime"`
	Tags          []string `json:"tags"`
}

type BatchMutationResponse struct {
	ID2Error map[string]string `json:"id2error"`
	ID2ETag  map[string]string `json:"id2etag"`
}

type BatchProjectRequest struct {
	Add    []ProjectCreateV2 `json:"add,omitempty"`
	Update []ProjectUpdateV2 `json:"update,omitempty"`
	Delete []string          `json:"delete,omitempty"`
}

type ProjectCreateV2 struct {
	Name string `json:"name"`
}

type ProjectUpdateV2 struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BatchTaskRequest struct {
	Add    []TaskCreateV2 `json:"add,omitempty"`
	Update []TaskUpdateV2 `json:"update,omitempty"`
	Delete []TaskDeleteV2 `json:"delete,omitempty"`
}

type TaskCreateV2 struct {
	ProjectID string   `json:"projectId"`
	Title     string   `json:"title"`
	Content   string   `json:"content,omitempty"`
	Priority  int      `json:"priority,omitempty"`
	DueDate   string   `json:"dueDate,omitempty"`
	StartDate string   `json:"startDate,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type TaskUpdateV2 struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"projectId"`
	Title     string   `json:"title"`
	Content   string   `json:"content,omitempty"`
	Priority  int      `json:"priority,omitempty"`
	Status    int      `json:"status,omitempty"`
	DueDate   string   `json:"dueDate,omitempty"`
	StartDate string   `json:"startDate,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type TaskDeleteV2 struct {
	TaskID    string `json:"taskId"`
	ProjectID string `json:"projectId"`
}

type OpenProject struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SortOrder  int64  `json:"sortOrder"`
	Permission string `json:"permission"`
	Kind       string `json:"kind"`
}

type OpenProjectData struct {
	Project OpenProject `json:"project"`
	Tasks   []OpenTask  `json:"tasks"`
}

type OpenTask struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"projectId"`
	SortOrder int64    `json:"sortOrder"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Desc      string   `json:"desc"`
	TimeZone  string   `json:"timeZone"`
	IsAllDay  bool     `json:"isAllDay"`
	Priority  int      `json:"priority"`
	Status    int      `json:"status"`
	Tags      []string `json:"tags"`
	ETag      string   `json:"etag"`
	Kind      string   `json:"kind"`
	DueDate   string   `json:"dueDate"`
	StartDate string   `json:"startDate"`
	DateTime  string   `json:"dateTime"`
}

type OpenProjectCreateRequest struct {
	Name string `json:"name"`
}

type OpenTaskCreateRequest struct {
	ProjectID string   `json:"projectId"`
	Title     string   `json:"title"`
	Content   string   `json:"content,omitempty"`
	Priority  int      `json:"priority,omitempty"`
	DueDate   string   `json:"dueDate,omitempty"`
	StartDate string   `json:"startDate,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type OpenTaskUpdateRequest struct {
	ProjectID string   `json:"projectId,omitempty"`
	Title     string   `json:"title,omitempty"`
	Content   string   `json:"content,omitempty"`
	Priority  int      `json:"priority,omitempty"`
	Status    int      `json:"status,omitempty"`
	DueDate   string   `json:"dueDate,omitempty"`
	StartDate string   `json:"startDate,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}
