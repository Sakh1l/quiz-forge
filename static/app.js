// Quiz Forge Frontend JavaScript

// SSE connection management
class QuizConnection {
    constructor(roomCode) {
        this.roomCode = roomCode;
        this.eventSource = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000;
    }

    connect() {
        if (this.eventSource) {
            this.eventSource.close();
        }

        this.eventSource = new EventSource(`/api/session/${this.roomCode}/events`);

        this.eventSource.onopen = () => {
            console.log('Connected to quiz session');
            this.reconnectAttempts = 0;
        };

        this.eventSource.onerror = (error) => {
            console.error('SSE connection error:', error);
            this.handleReconnect();
        };

        this.eventSource.addEventListener('player_joined', (event) => {
            const data = JSON.parse(event.data);
            this.updatePlayerCount(data.count);
            this.showNotification(`${data.nickname} joined the quiz`, 'success');
        });

        this.eventSource.addEventListener('player_left', (event) => {
            const data = JSON.parse(event.data);
            this.updatePlayerCount(data.count);
            this.showNotification(`${data.nickname} left the quiz`, 'info');
        });

        this.eventSource.addEventListener('question_start', (event) => {
            const data = JSON.parse(event.data);
            this.displayQuestion(data);
        });

        this.eventSource.addEventListener('question_end', (event) => {
            const data = JSON.parse(event.data);
            this.displayResults(data);
        });

        this.eventSource.addEventListener('timer_tick', (event) => {
            const data = JSON.parse(event.data);
            this.updateTimer(data.remaining);
        });

        this.eventSource.addEventListener('session_ended', (event) => {
            const data = JSON.parse(event.data);
            this.displayFinalResults(data);
        });
    }

    handleReconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            console.log(`Attempting to reconnect (${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
            setTimeout(() => this.connect(), this.reconnectDelay * this.reconnectAttempts);
        } else {
            console.error('Max reconnection attempts reached');
            this.showNotification('Connection lost. Please refresh the page.', 'error');
        }
    }

    disconnect() {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }
    }

    updatePlayerCount(count) {
        const elements = document.querySelectorAll('[data-player-count]');
        elements.forEach(el => el.textContent = count);
    }

    showNotification(message, type = 'info') {
        const notification = document.createElement('div');
        notification.className = `notification notification-${type}`;
        notification.textContent = message;
        
        const container = document.getElementById('notifications') || document.body;
        container.appendChild(notification);

        setTimeout(() => {
            notification.remove();
        }, 3000);
    }

    displayQuestion(data) {
        // Update question display
        const questionText = document.querySelector('[data-question-text]');
        if (questionText) {
            questionText.textContent = data.text;
        }

        // Update answer options
        const answersContainer = document.querySelector('[data-answers]');
        if (answersContainer) {
            answersContainer.innerHTML = '';
            data.answers.forEach((answer, index) => {
                const button = document.createElement('button');
                button.className = 'answer-btn';
                button.textContent = answer;
                button.onclick = () => this.submitAnswer(index);
                answersContainer.appendChild(button);
            });
        }

        // Show question area
        document.querySelectorAll('[data-question-area]').forEach(el => {
            el.style.display = 'block';
        });
    }

    displayResults(data) {
        // Disable answer buttons
        document.querySelectorAll('.answer-btn').forEach(btn => {
            btn.disabled = true;
            btn.classList.add('disabled');
        });

        // Show correct answer
        const correctIndex = data.correctIndex;
        const buttons = document.querySelectorAll('.answer-btn');
        if (buttons[correctIndex]) {
            buttons[correctIndex].classList.add('correct');
        }

        // Show statistics
        const statsContainer = document.querySelector('[data-stats]');
        if (statsContainer && data.stats) {
            statsContainer.innerHTML = '';
            data.stats.forEach((stat, index) => {
                const statEl = document.createElement('div');
                statEl.className = 'stat-item';
                statEl.innerHTML = `
                    <span>${String.fromCharCode(65 + index)}: ${stat.count} votes</span>
                    <div class="stat-bar" style="width: ${stat.percentage}%"></div>
                `;
                statsContainer.appendChild(statEl);
            });
        }
    }

    updateTimer(remaining) {
        const timerElements = document.querySelectorAll('[data-timer]');
        timerElements.forEach(el => {
            el.textContent = remaining;
            if (remaining <= 5) {
                el.classList.add('timer-critical');
            } else {
                el.classList.remove('timer-critical');
            }
        });
    }

    displayFinalResults(data) {
        const leaderboard = document.querySelector('[data-leaderboard]');
        if (leaderboard && data.rankings) {
            leaderboard.innerHTML = '';
            data.rankings.forEach((player, index) => {
                const row = document.createElement('tr');
                row.innerHTML = `
                    <td>${index + 1}</td>
                    <td>${player.nickname}</td>
                    <td>${player.score}</td>
                    <td>${Math.round(player.total_time / 1000)}s</td>
                `;
                leaderboard.appendChild(row);
            });
        }

        // Show final results section
        document.querySelectorAll('[data-final-results]').forEach(el => {
            el.style.display = 'block';
        });
    }

    submitAnswer(answerIndex) {
        const form = document.querySelector('#answer-form');
        if (form) {
            const input = form.querySelector('input[name="answer"]');
            if (input) {
                input.value = answerIndex;
                form.submit();
            }
        }
    }
}

// Initialize quiz connection when page loads
document.addEventListener('DOMContentLoaded', () => {
    const roomCode = document.body.dataset.roomCode;
    if (roomCode) {
        const connection = new QuizConnection(roomCode);
        connection.connect();

        // Cleanup on page unload
        window.addEventListener('beforeunload', () => {
            connection.disconnect();
        });
    }
});

// HTMX enhancements
document.addEventListener('htmx:afterRequest', (event) => {
    // Re-initialize any components after HTMX updates
    const target = event.target;
    
    // Re-bind answer buttons
    target.querySelectorAll('.answer-btn').forEach(btn => {
        if (!btn.onclick) {
            const answerIndex = Array.from(btn.parentElement.children).indexOf(btn);
            btn.onclick = () => {
                const form = document.querySelector('#answer-form');
                if (form) {
                    const input = form.querySelector('input[name="answer"]');
                    if (input) {
                        input.value = answerIndex;
                        form.submit();
                    }
                }
            };
        }
    });
});

// Utility functions
function formatTime(seconds) {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

function generateQRCode(text) {
    // This would integrate with a QR code library
    // For now, just return a placeholder
    return `QR Code for: ${text}`;
}

// Auto-refresh functionality for host dashboard
function startAutoRefresh(interval = 5000) {
    const refreshElements = document.querySelectorAll('[data-auto-refresh]');
    if (refreshElements.length > 0) {
        setInterval(() => {
            refreshElements.forEach(el => {
                if (el.dataset.autoRefresh) {
                    htmx.trigger(el, 'refresh');
                }
            });
        }, interval);
    }
}
