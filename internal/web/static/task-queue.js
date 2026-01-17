/**
 * TaskQueue - Unified client-side interface for async task management
 * Provides task submission, polling, tracking, and UI status display
 */
(function() {
    'use strict';

    const POLL_INTERVAL = 1000; // Poll every 1 second when active
    const IDLE_TIMEOUT = 5000; // Stop polling 5 seconds after no active tasks
    const MAX_COMPLETED_DISPLAY = 5; // Show last 5 completed tasks

    const TASK_TYPES = [
        'install', 'delete', 'convert', 'export',
        'ingest', 'verify', 'selfcheck', 'tool_run'
    ];

    class TaskQueue {
        constructor() {
            this.tasks = new Map(); // taskId -> task object
            this.pollTimer = null;
            this.idleTimer = null;
            this.isPolling = false;
            this.statusPanel = null;
            this.listeners = new Map(); // event name -> array of callbacks

            this.init();
        }

        init() {
            // Lazy initialization - panel and styles created on first task
            this.initialized = false;
        }

        ensureInitialized() {
            if (!this.initialized) {
                this.createStatusPanel();
                this.initialized = true;
            }
        }

        /**
         * Submit a new task
         * @param {string} type - Task type (install, delete, etc.)
         * @param {string} name - Task name/description
         * @param {Object} params - Additional parameters
         * @returns {Promise<Object>} Task response
         */
        async submitTask(type, name, params = {}) {
            if (!TASK_TYPES.includes(type)) {
                throw new Error(`Invalid task type: ${type}. Must be one of: ${TASK_TYPES.join(', ')}`);
            }

            const formData = new FormData();
            formData.append('type', type);
            formData.append('name', name);

            // Add additional parameters
            for (const [key, value] of Object.entries(params)) {
                if (key !== 'type' && key !== 'name') {
                    formData.append(key, value);
                }
            }

            try {
                const response = await fetch('/api/tasks/add', {
                    method: 'POST',
                    body: formData
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    throw new Error(`Task submission failed: ${errorText}`);
                }

                const task = await response.json();

                // Ensure panel exists before first task
                this.ensureInitialized();

                // Add to local tracking
                this.tasks.set(task.id, {
                    id: task.id,
                    type: type,
                    name: name,
                    status: 'queued',
                    progress: 0,
                    message: '',
                    timestamp: Date.now()
                });

                this.emit('taskSubmitted', task);
                this.startPolling();
                this.updateUI();

                return task;
            } catch (error) {
                console.error('Error submitting task:', error);
                this.emit('taskError', { type, name, error: error.message });
                throw error;
            }
        }

        /**
         * Poll for task status updates
         * @param {Object} filters - Optional filters (type, id)
         */
        async pollStatus(filters = {}) {
            try {
                const params = new URLSearchParams();
                if (filters.type) params.append('type', filters.type);
                if (filters.id) params.append('id', filters.id);

                const url = `/api/tasks/status${params.toString() ? '?' + params.toString() : ''}`;
                const response = await fetch(url);

                if (!response.ok) {
                    throw new Error(`Status poll failed: ${response.statusText}`);
                }

                const tasks = await response.json();
                this.updateTasks(tasks);
                this.updateUI();

                return tasks;
            } catch (error) {
                console.error('Error polling status:', error);
                // Don't stop polling on error, just log it
            }
        }

        /**
         * Update internal task tracking with server response
         */
        updateTasks(serverTasks) {
            if (!Array.isArray(serverTasks)) {
                serverTasks = [serverTasks];
            }

            serverTasks.forEach(serverTask => {
                const existingTask = this.tasks.get(serverTask.id);
                const wasRunning = existingTask && existingTask.status === 'running';
                const wasQueued = existingTask && existingTask.status === 'queued';

                this.tasks.set(serverTask.id, {
                    ...existingTask,
                    ...serverTask,
                    timestamp: existingTask ? existingTask.timestamp : Date.now()
                });

                // Emit events for status changes
                if (!existingTask && serverTask.status === 'running') {
                    this.emit('taskStarted', serverTask);
                } else if (wasQueued && serverTask.status === 'running') {
                    this.emit('taskStarted', serverTask);
                } else if (wasRunning && serverTask.status === 'completed') {
                    this.emit('taskCompleted', serverTask);
                } else if (wasRunning && serverTask.status === 'failed') {
                    this.emit('taskFailed', serverTask);
                }
            });

            // Check if we should stop polling
            if (this.getActiveTasks().length === 0) {
                this.scheduleIdleStop();
            }
        }

        /**
         * Get tasks by status
         */
        getTasksByStatus(status) {
            return Array.from(this.tasks.values()).filter(task => task.status === status);
        }

        getActiveTasks() {
            return Array.from(this.tasks.values()).filter(
                task => task.status === 'running' || task.status === 'queued'
            );
        }

        getCompletedTasks() {
            return Array.from(this.tasks.values())
                .filter(task => task.status === 'completed' || task.status === 'failed')
                .sort((a, b) => (b.timestamp || 0) - (a.timestamp || 0))
                .slice(0, MAX_COMPLETED_DISPLAY);
        }

        /**
         * Start polling for updates
         */
        startPolling() {
            if (this.isPolling) return;

            this.isPolling = true;
            this.clearIdleTimer();

            const poll = async () => {
                await this.pollStatus();
                if (this.isPolling) {
                    this.pollTimer = setTimeout(poll, POLL_INTERVAL);
                }
            };

            poll();
        }

        /**
         * Stop polling
         */
        stopPolling() {
            this.isPolling = false;
            if (this.pollTimer) {
                clearTimeout(this.pollTimer);
                this.pollTimer = null;
            }
            this.clearIdleTimer();
        }

        /**
         * Schedule automatic stop when idle
         */
        scheduleIdleStop() {
            this.clearIdleTimer();
            this.idleTimer = setTimeout(() => {
                if (this.getActiveTasks().length === 0) {
                    this.stopPolling();
                    this.emit('idle');
                }
            }, IDLE_TIMEOUT);
        }

        clearIdleTimer() {
            if (this.idleTimer) {
                clearTimeout(this.idleTimer);
                this.idleTimer = null;
            }
        }

        /**
         * Create the floating status panel UI
         */
        createStatusPanel() {
            const panel = document.createElement('div');
            panel.id = 'task-queue-panel';
            panel.className = 'task-queue-panel';
            panel.innerHTML = `
                <div class="task-queue-header">
                    <span class="task-queue-title">Tasks</span>
                    <button class="task-queue-close" title="Hide">&times;</button>
                </div>
                <div class="task-queue-body">
                    <div class="task-queue-running"></div>
                    <div class="task-queue-queued"></div>
                    <div class="task-queue-completed"></div>
                </div>
            `;

            document.body.appendChild(panel);
            this.statusPanel = panel;

            // Close button handler
            const closeBtn = panel.querySelector('.task-queue-close');
            closeBtn.addEventListener('click', () => {
                this.hidePanel();
            });

            // Initially hidden
            this.hidePanel();
        }

        /**
         * Update the status panel UI
         */
        updateUI() {
            if (!this.statusPanel) return;

            const runningTasks = this.getTasksByStatus('running');
            const queuedTasks = this.getTasksByStatus('queued');
            const completedTasks = this.getCompletedTasks();

            const runningContainer = this.statusPanel.querySelector('.task-queue-running');
            const queuedContainer = this.statusPanel.querySelector('.task-queue-queued');
            const completedContainer = this.statusPanel.querySelector('.task-queue-completed');

            // Show panel if there are any tasks
            if (runningTasks.length > 0 || queuedTasks.length > 0 || completedTasks.length > 0) {
                this.showPanel();
            } else {
                this.hidePanel();
                return;
            }

            // Running tasks
            if (runningTasks.length > 0) {
                runningContainer.innerHTML = runningTasks.map(task => `
                    <div class="task-item task-running" data-task-id="${task.id}">
                        <div class="task-spinner"></div>
                        <div class="task-info">
                            <div class="task-name">${this.escapeHtml(task.name)}</div>
                            <div class="task-message">${this.escapeHtml(task.message || 'Running...')}</div>
                            ${task.progress ? `<div class="task-progress"><div class="task-progress-bar" style="width: ${task.progress}%"></div></div>` : ''}
                        </div>
                    </div>
                `).join('');
            } else {
                runningContainer.innerHTML = '';
            }

            // Queued tasks
            if (queuedTasks.length > 0) {
                queuedContainer.innerHTML = `
                    <div class="task-queued-summary">
                        <span class="task-queued-icon">&#9203;</span>
                        ${queuedTasks.length} task${queuedTasks.length > 1 ? 's' : ''} queued
                    </div>
                `;
            } else {
                queuedContainer.innerHTML = '';
            }

            // Completed tasks
            if (completedTasks.length > 0) {
                completedContainer.innerHTML = completedTasks.map(task => `
                    <div class="task-item task-${task.status}" data-task-id="${task.id}">
                        <div class="task-status-icon">${task.status === 'completed' ? '&#10003;' : '&#10007;'}</div>
                        <div class="task-info">
                            <div class="task-name">${this.escapeHtml(task.name)}</div>
                            ${task.message ? `<div class="task-message">${this.escapeHtml(task.message)}</div>` : ''}
                        </div>
                    </div>
                `).join('');

                // Add click handlers for completed tasks
                completedContainer.querySelectorAll('.task-item').forEach(item => {
                    item.addEventListener('click', () => {
                        const taskId = item.dataset.taskId;
                        const task = this.tasks.get(taskId);
                        if (task) {
                            this.showTaskDetails(task);
                        }
                    });
                });
            } else {
                completedContainer.innerHTML = '';
            }
        }

        showPanel() {
            if (this.statusPanel) {
                this.statusPanel.classList.add('visible');
            }
        }

        hidePanel() {
            if (this.statusPanel) {
                this.statusPanel.classList.remove('visible');
            }
        }

        /**
         * Show detailed information about a task
         */
        showTaskDetails(task) {
            const details = `
Task: ${task.name}
Type: ${task.type}
Status: ${task.status}
${task.message ? 'Message: ' + task.message : ''}
${task.progress ? 'Progress: ' + task.progress + '%' : ''}
            `.trim();

            alert(details);
            this.emit('taskDetailsShown', task);
        }

        /**
         * Intercept form submissions and convert to task submissions
         * @param {string} formSelector - CSS selector for the form
         * @param {string} taskType - Task type
         * @param {Function} nameExtractor - Function to extract task name from form data
         */
        interceptForm(formSelector, taskType, nameExtractor) {
            const form = document.querySelector(formSelector);
            if (!form) {
                console.warn(`Form not found: ${formSelector}`);
                return;
            }

            form.addEventListener('submit', async (e) => {
                e.preventDefault();

                const formData = new FormData(form);
                const params = {};

                // Convert FormData to object
                for (const [key, value] of formData.entries()) {
                    params[key] = value;
                }

                // Extract task name
                let taskName;
                if (typeof nameExtractor === 'function') {
                    taskName = nameExtractor(params, form);
                } else if (typeof nameExtractor === 'string') {
                    taskName = params[nameExtractor] || 'Task';
                } else {
                    taskName = params.name || params.id || 'Task';
                }

                // Find submit button
                const submitBtn = form.querySelector('button[type="submit"], input[type="submit"]');
                const originalText = submitBtn ? submitBtn.textContent || submitBtn.value : '';

                try {
                    // Update button state
                    if (submitBtn) {
                        submitBtn.disabled = true;
                        if (submitBtn.tagName === 'BUTTON') {
                            submitBtn.textContent = 'Queued...';
                        } else {
                            submitBtn.value = 'Queued...';
                        }
                    }

                    // Submit as task
                    await this.submitTask(taskType, taskName, params);

                    // Reset form
                    form.reset();

                    // Listen for task completion to re-enable button
                    const onComplete = (task) => {
                        if (task.name === taskName && submitBtn) {
                            submitBtn.disabled = false;
                            if (submitBtn.tagName === 'BUTTON') {
                                submitBtn.textContent = originalText;
                            } else {
                                submitBtn.value = originalText;
                            }
                            this.off('taskCompleted', onComplete);
                            this.off('taskFailed', onComplete);
                        }
                    };

                    this.on('taskCompleted', onComplete);
                    this.on('taskFailed', onComplete);

                } catch (error) {
                    // Re-enable button on error
                    if (submitBtn) {
                        submitBtn.disabled = false;
                        if (submitBtn.tagName === 'BUTTON') {
                            submitBtn.textContent = originalText;
                        } else {
                            submitBtn.value = originalText;
                        }
                    }
                    console.error('Error submitting task:', error);
                    alert('Error submitting task: ' + error.message);
                }
            });
        }

        /**
         * Intercept multiple forms matching a selector and convert to task submissions.
         * Uses event delegation for performance with many forms.
         * @param {string} formSelector - CSS selector for forms (can match multiple)
         * @param {string} taskType - Task type
         * @param {Function} nameExtractor - Function to extract task name from form element
         */
        interceptForms(formSelector, taskType, nameExtractor) {
            // Store configuration for delegation
            if (!this.formInterceptors) {
                this.formInterceptors = [];
                // Set up single delegated listener on document
                document.addEventListener('submit', (e) => {
                    const form = e.target;
                    if (form.tagName !== 'FORM') return;

                    // Check if form matches any interceptor
                    for (const interceptor of this.formInterceptors) {
                        if (form.matches(interceptor.selector)) {
                            e.preventDefault();
                            this.handleFormSubmit(form, interceptor.taskType, interceptor.nameExtractor);
                            return;
                        }
                    }
                });
            }

            // Add this interceptor to the list
            this.formInterceptors.push({
                selector: formSelector,
                taskType: taskType,
                nameExtractor: nameExtractor
            });
        }

        /**
         * Handle form submission as async task
         */
        async handleFormSubmit(form, taskType, nameExtractor) {
            const formData = new FormData(form);
            const params = {};

            // Convert FormData to object
            for (const [key, value] of formData.entries()) {
                params[key] = value;
            }

            // Extract task name
            let taskName;
            if (typeof nameExtractor === 'function') {
                taskName = nameExtractor(form);
            } else if (typeof nameExtractor === 'string') {
                taskName = params[nameExtractor] || 'Task';
            } else {
                taskName = params.name || params.id || 'Task';
            }

            // Find submit button
            const submitBtn = form.querySelector('button[type="submit"], input[type="submit"]');
            const originalText = submitBtn ? submitBtn.textContent || submitBtn.value : '';

            try {
                // Update button state
                if (submitBtn) {
                    submitBtn.disabled = true;
                    if (submitBtn.tagName === 'BUTTON') {
                        submitBtn.textContent = 'Queued...';
                    } else {
                        submitBtn.value = 'Queued...';
                    }
                }

                // Submit as task
                await this.submitTask(taskType, taskName, params);

                // Listen for task completion to re-enable button
                const onComplete = (task) => {
                    if (task.name === taskName && submitBtn) {
                        submitBtn.disabled = false;
                        if (submitBtn.tagName === 'BUTTON') {
                            submitBtn.textContent = originalText;
                        } else {
                            submitBtn.value = originalText;
                        }
                        this.off('taskCompleted', onComplete);
                        this.off('taskFailed', onComplete);
                    }
                };

                this.on('taskCompleted', onComplete);
                this.on('taskFailed', onComplete);

            } catch (error) {
                // Re-enable button on error
                if (submitBtn) {
                    submitBtn.disabled = false;
                    if (submitBtn.tagName === 'BUTTON') {
                        submitBtn.textContent = originalText;
                    } else {
                        submitBtn.value = originalText;
                    }
                }
                console.error('Error submitting task:', error);
                alert('Error submitting task: ' + error.message);
            }
        }

        /**
         * Event system
         */
        on(eventName, callback) {
            if (!this.listeners.has(eventName)) {
                this.listeners.set(eventName, []);
            }
            this.listeners.get(eventName).push(callback);
        }

        off(eventName, callback) {
            if (!this.listeners.has(eventName)) return;
            const callbacks = this.listeners.get(eventName);
            const index = callbacks.indexOf(callback);
            if (index > -1) {
                callbacks.splice(index, 1);
            }
        }

        emit(eventName, data) {
            if (!this.listeners.has(eventName)) return;
            this.listeners.get(eventName).forEach(callback => {
                try {
                    callback(data);
                } catch (error) {
                    console.error(`Error in event listener for ${eventName}:`, error);
                }
            });
        }

        /**
         * Utility: Escape HTML
         */
        escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        /**
         * Clear all completed tasks from display
         */
        clearCompleted() {
            const completedTasks = this.getCompletedTasks();
            completedTasks.forEach(task => {
                this.tasks.delete(task.id);
            });
            this.updateUI();
        }

        /**
         * Get task by ID
         */
        getTask(taskId) {
            return this.tasks.get(taskId);
        }

        /**
         * Get all tasks
         */
        getAllTasks() {
            return Array.from(this.tasks.values());
        }
    }

    // Create singleton instance and expose globally
    window.TaskQueue = new TaskQueue();
})();
