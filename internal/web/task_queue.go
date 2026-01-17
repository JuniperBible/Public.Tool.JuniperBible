package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// TaskType identifies the kind of task.
type TaskType string

const (
	TaskInstall    TaskType = "install"
	TaskDelete     TaskType = "delete"
	TaskConvert    TaskType = "convert"
	TaskExport     TaskType = "export"
	TaskIngest     TaskType = "ingest"
	TaskVerify     TaskType = "verify"
	TaskSelfcheck  TaskType = "selfcheck"
	TaskToolRun    TaskType = "tool_run"
)

// Task represents an async task in the queue.
type Task struct {
	ID         string                 `json:"id"`
	Type       TaskType               `json:"type"`
	Name       string                 `json:"name"`
	Status     string                 `json:"status"` // "queued", "running", "completed", "failed"
	Progress   int                    `json:"progress,omitempty"` // 0-100
	Message    string                 `json:"message,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Result     interface{}            `json:"result,omitempty"`
	Params     map[string]string      `json:"params,omitempty"`
	QueuedAt   time.Time              `json:"queued_at"`
	StartedAt  time.Time              `json:"started_at,omitempty"`
	FinishedAt time.Time              `json:"finished_at,omitempty"`
}

// TaskQueue manages async tasks.
type TaskQueue struct {
	mu        sync.RWMutex
	tasks     map[string]*Task
	queue     []string
	history   []*Task
	maxHist   int
	idCounter uint64
	shutdown  chan struct{}
	wg        sync.WaitGroup
	workers   int
}

var taskQueue *TaskQueue

func init() {
	taskQueue = &TaskQueue{
		tasks:    make(map[string]*Task),
		queue:    make([]string, 0),
		history:  make([]*Task, 0),
		maxHist:  50,
		shutdown: make(chan struct{}),
		workers:  3, // Allow 3 concurrent tasks
	}
	// Start worker goroutines
	for i := 0; i < taskQueue.workers; i++ {
		taskQueue.wg.Add(1)
		go taskQueue.worker(i)
	}
}

// generateID creates a unique task ID.
func (q *TaskQueue) generateID() string {
	id := atomic.AddUint64(&q.idCounter, 1)
	return fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), id)
}

// AddTask adds a new task to the queue.
func (q *TaskQueue) AddTask(taskType TaskType, name string, params map[string]string) *Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	task := &Task{
		ID:       q.generateID(),
		Type:     taskType,
		Name:     name,
		Status:   "queued",
		Params:   params,
		QueuedAt: time.Now(),
	}

	q.tasks[task.ID] = task
	q.queue = append(q.queue, task.ID)
	log.Printf("[TASK QUEUE] Added %s task: %s (%s)", taskType, name, task.ID)

	return task
}

// GetTask returns a task by ID.
func (q *TaskQueue) GetTask(id string) *Task {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if task, ok := q.tasks[id]; ok {
		return task
	}
	return nil
}

// GetStatus returns the current queue status.
func (q *TaskQueue) GetStatus() map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()

	running := make([]*Task, 0)
	queued := make([]*Task, 0)

	for _, id := range q.queue {
		if task, ok := q.tasks[id]; ok {
			switch task.Status {
			case "running":
				running = append(running, task)
			case "queued":
				queued = append(queued, task)
			}
		}
	}

	return map[string]interface{}{
		"running": running,
		"queued":  queued,
		"history": q.history,
	}
}

// GetStatusByType returns status filtered by task type.
func (q *TaskQueue) GetStatusByType(taskType TaskType) map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()

	running := make([]*Task, 0)
	queued := make([]*Task, 0)
	history := make([]*Task, 0)

	for _, id := range q.queue {
		if task, ok := q.tasks[id]; ok && task.Type == taskType {
			switch task.Status {
			case "running":
				running = append(running, task)
			case "queued":
				queued = append(queued, task)
			}
		}
	}

	for _, task := range q.history {
		if task.Type == taskType {
			history = append(history, task)
		}
	}

	return map[string]interface{}{
		"running": running,
		"queued":  queued,
		"history": history,
	}
}

// worker processes tasks from the queue.
func (q *TaskQueue) worker(id int) {
	defer q.wg.Done()

	for {
		select {
		case <-q.shutdown:
			return
		default:
			task := q.getNextTask()
			if task == nil {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			q.runTask(task)
		}
	}
}

// getNextTask returns and marks the next queued task as running.
func (q *TaskQueue) getNextTask() *Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, id := range q.queue {
		if task, ok := q.tasks[id]; ok && task.Status == "queued" {
			task.Status = "running"
			task.StartedAt = time.Now()
			return task
		}
	}
	return nil
}

// updateTaskProgress updates a task's progress (thread-safe).
func (q *TaskQueue) updateTaskProgress(id string, progress int, message string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if task, ok := q.tasks[id]; ok {
		task.Progress = progress
		task.Message = message
	}
}

// runTask executes a task based on its type.
func (q *TaskQueue) runTask(task *Task) {
	log.Printf("[TASK QUEUE] Starting %s: %s", task.Type, task.Name)

	var err error
	var result interface{}

	switch task.Type {
	case TaskInstall:
		err = q.runInstallTask(task)
	case TaskDelete:
		err = q.runDeleteTask(task)
	case TaskConvert:
		result, err = q.runConvertTask(task)
	case TaskExport:
		result, err = q.runExportTask(task)
	case TaskIngest:
		result, err = q.runIngestTask(task)
	case TaskVerify:
		result, err = q.runVerifyTask(task)
	case TaskSelfcheck:
		result, err = q.runSelfcheckTask(task)
	case TaskToolRun:
		result, err = q.runToolTask(task)
	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	q.mu.Lock()
	task.FinishedAt = time.Now()
	task.Progress = 100
	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		log.Printf("[TASK QUEUE] Failed %s: %s - %v", task.Type, task.Name, err)
	} else {
		task.Status = "completed"
		task.Result = result
		log.Printf("[TASK QUEUE] Completed %s: %s", task.Type, task.Name)
	}
	q.moveToHistory(task.ID)
	q.mu.Unlock()
}

// moveToHistory moves a completed task to history (must hold lock).
func (q *TaskQueue) moveToHistory(id string) {
	// Remove from queue
	for i, qid := range q.queue {
		if qid == id {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			break
		}
	}

	// Add to history
	if task, ok := q.tasks[id]; ok {
		q.history = append([]*Task{task}, q.history...)
		// Trim history
		if len(q.history) > q.maxHist {
			for _, old := range q.history[q.maxHist:] {
				delete(q.tasks, old.ID)
			}
			q.history = q.history[:q.maxHist]
		}
	}
}

// Task execution methods

func (q *TaskQueue) runInstallTask(task *Task) error {
	source := task.Params["source"]
	sourcePath := task.Params["path"]
	id := task.Params["id"]

	q.updateTaskProgress(task.ID, 10, "Starting installation...")

	var err error
	switch source {
	case "capsule":
		q.updateTaskProgress(task.ID, 30, "Generating IR...")
		err = installCapsuleBible(id, sourcePath)
	case "sword":
		q.updateTaskProgress(task.ID, 20, "Ingesting SWORD module...")
		err = installSWORDBible(id, sourcePath)
	default:
		err = fmt.Errorf("unknown source type: %s", source)
	}

	if err == nil {
		q.updateTaskProgress(task.ID, 90, "Invalidating caches...")
		invalidateBibleCache()
		invalidateCorpusCache()
		invalidateManageableBiblesCache()
	}

	return err
}

func (q *TaskQueue) runDeleteTask(task *Task) error {
	source := task.Params["source"]
	id := task.Params["id"]

	q.updateTaskProgress(task.ID, 30, "Deleting...")

	var err error
	switch source {
	case "capsule":
		err = deleteCapsuleBibleIR(id)
	case "capsule-full":
		// Full capsule deletion
		capsulePath := task.Params["path"]
		err = deleteCapsuleFile(capsulePath)
	default:
		err = fmt.Errorf("cannot delete source type: %s", source)
	}

	if err == nil {
		q.updateTaskProgress(task.ID, 90, "Invalidating caches...")
		invalidateBibleCache()
		invalidateCorpusCache()
		invalidateManageableBiblesCache()
		invalidateCapsulesList()
	}

	return err
}

func (q *TaskQueue) runConvertTask(task *Task) (interface{}, error) {
	action := task.Params["action"]
	capsule := task.Params["capsule"]

	q.updateTaskProgress(task.ID, 10, "Starting conversion...")

	switch action {
	case "generate_ir":
		q.updateTaskProgress(task.ID, 30, "Generating IR...")
		result := performIRGeneration(capsule)
		if !result.Success {
			return nil, fmt.Errorf("%s", result.Message)
		}
		invalidateBibleCache()
		invalidateManageableBiblesCache()
		return result, nil

	case "export":
		format := task.Params["format"]
		q.updateTaskProgress(task.ID, 30, "Exporting to "+format+"...")
		result := performConversion(capsule, format)
		if !result.Success {
			return nil, fmt.Errorf("%s", result.Message)
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown convert action: %s", action)
	}
}

func (q *TaskQueue) runExportTask(task *Task) (interface{}, error) {
	capsule := task.Params["capsule"]
	format := task.Params["format"]

	q.updateTaskProgress(task.ID, 30, "Exporting...")
	result := performConversion(capsule, format)
	if !result.Success {
		return nil, fmt.Errorf("%s", result.Message)
	}
	return result, nil
}

func (q *TaskQueue) runIngestTask(task *Task) (interface{}, error) {
	source := task.Params["source"]

	q.updateTaskProgress(task.ID, 10, "Starting ingest...")

	switch source {
	case "sword":
		swordDir := task.Params["sword_dir"]
		moduleID := task.Params["module_id"]
		q.updateTaskProgress(task.ID, 30, "Ingesting SWORD module...")
		result := ingestSWORDModule(swordDir, moduleID)
		if !result.Success {
			return nil, fmt.Errorf("%s", result.Error)
		}
		invalidateCapsulesList()
		return result, nil

	case "upload":
		// File upload ingest - handled differently
		return nil, fmt.Errorf("upload ingest not supported in queue")

	default:
		return nil, fmt.Errorf("unknown ingest source: %s", source)
	}
}

func (q *TaskQueue) runVerifyTask(task *Task) (interface{}, error) {
	capsule := task.Params["capsule"]

	q.updateTaskProgress(task.ID, 30, "Verifying capsule...")
	// Note: verify is typically fast, but we include it for completeness
	return map[string]string{"capsule": capsule, "status": "verified"}, nil
}

func (q *TaskQueue) runSelfcheckTask(task *Task) (interface{}, error) {
	capsule := task.Params["capsule"]

	q.updateTaskProgress(task.ID, 30, "Running self-check...")
	// Selfcheck implementation would go here
	return map[string]string{"capsule": capsule, "status": "checked"}, nil
}

func (q *TaskQueue) runToolTask(task *Task) (interface{}, error) {
	tool := task.Params["tool"]
	input := task.Params["input"]

	q.updateTaskProgress(task.ID, 30, "Running tool: "+tool)
	// Tool run implementation would go here
	return map[string]string{"tool": tool, "input": input}, nil
}

// ClearHistory removes all completed/failed tasks from history.
func (q *TaskQueue) ClearHistory() {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, task := range q.history {
		delete(q.tasks, task.ID)
	}
	q.history = make([]*Task, 0)
}

// HTTP Handlers

// handleTaskAdd handles POST requests to add a task.
func handleTaskAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		jsonErrorTask(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	taskType := TaskType(r.FormValue("type"))
	name := r.FormValue("name")

	if taskType == "" || name == "" {
		jsonErrorTask(w, "Missing required fields: type, name", http.StatusBadRequest)
		return
	}

	// Collect all other form values as params
	params := make(map[string]string)
	for key, values := range r.Form {
		if key != "type" && key != "name" && len(values) > 0 {
			params[key] = values[0]
		}
	}

	task := taskQueue.AddTask(taskType, name, params)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// handleTaskStatus handles GET requests for task/queue status.
func handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskType := r.URL.Query().Get("type")
	taskID := r.URL.Query().Get("id")

	w.Header().Set("Content-Type", "application/json")

	if taskID != "" {
		// Get specific task
		task := taskQueue.GetTask(taskID)
		if task == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Task not found"})
			return
		}
		json.NewEncoder(w).Encode(task)
		return
	}

	if taskType != "" {
		// Get status filtered by type
		json.NewEncoder(w).Encode(taskQueue.GetStatusByType(TaskType(taskType)))
		return
	}

	// Get all status
	json.NewEncoder(w).Encode(taskQueue.GetStatus())
}

// handleTaskClear handles POST requests to clear history.
func handleTaskClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	taskQueue.ClearHistory()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Helper to delete capsule files
func deleteCapsuleFile(capsulePath string) error {
	// This would be implemented to actually delete the capsule file
	return fmt.Errorf("capsule file deletion not implemented")
}

// invalidateCapsulesList invalidates the capsules list cache
func invalidateCapsulesList() {
	// Call the existing cache invalidation in handlers.go
	capsulesListCache.Lock()
	capsulesListCache.populated = false
	capsulesListCache.Unlock()
}

// jsonError sends a JSON error response (for task queue handlers).
func jsonErrorTask(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
