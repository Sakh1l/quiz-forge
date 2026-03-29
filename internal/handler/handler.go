package handler

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/quizforge/quiz-forge/internal/config"
	"github.com/quizforge/quiz-forge/internal/models"
	"github.com/quizforge/quiz-forge/internal/repository/memory"
	"github.com/quizforge/quiz-forge/internal/service"
	"github.com/quizforge/quiz-forge/internal/sse"
)

//go:embed templates
var templateFiles embed.FS

type Handler struct {
	store          *memory.MemoryStore
	quizService    *service.QuizService
	sessionService *service.SessionService
	broker         *sse.Broker
	cfg            *config.Config
	templates      *template.Template
	timerMgr       *TimerManager
}

func NewHandler(store *memory.MemoryStore, broker *sse.Broker, cfg *config.Config) *Handler {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"eq":  func(a, b interface{}) bool { return a == b },
		"char": func(i int) string {
			return string([]byte{byte('A' + i)})
		},
		"str": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},
	}

	tmpl := template.New("").Funcs(funcMap)

	templateNames := []string{
		"templates/index.html",
		"templates/host/dashboard.html",
		"templates/host/editor.html",
		"templates/host/session.html",
		"templates/play/join.html",
		"templates/play/session.html",
		"templates/partials/question.html",
		"templates/partials/leaderboard.html",
		"templates/partials/reveal.html",
		"templates/partials/host_main.html",
		"templates/partials/host_controls.html",
		"templates/partials/players_panel.html",
		"templates/partials/play_main.html",
		"templates/base.html",
	}

	for _, name := range templateNames {
		data, err := templateFiles.ReadFile(name)
		if err != nil {
			log.Printf("Error reading template %s: %v", name, err)
			continue
		}
		_, err = tmpl.New(name).Parse(string(data))
		if err != nil {
			log.Printf("Error parsing template %s: %v", name, err)
		}
	}

	h := &Handler{
		store:          store,
		quizService:    service.NewQuizService(store),
		sessionService: service.NewSessionService(store, store),
		broker:         broker,
		cfg:            cfg,
		templates:      tmpl,
	}
	h.timerMgr = NewTimerManager(h.timerEndRound)
	return h
}

func generateRoomCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 6)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		code[i] = charset[n.Int64()]
	}
	return string(code)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return string(b)
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	quizzes := h.store.ListQuizzes()
	h.templates.ExecuteTemplate(w, "templates/index.html", map[string]interface{}{
		"Quizzes": quizzes,
	})
}

func (h *Handler) HostDashboard(w http.ResponseWriter, r *http.Request) {
	quizzes := h.store.ListQuizzes()
	h.templates.ExecuteTemplate(w, "templates/host/dashboard.html", map[string]interface{}{
		"Quizzes": quizzes,
	})
}

func (h *Handler) CreateQuiz(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	description := r.FormValue("description")

	quiz := models.NewQuiz(title, description)
	quiz.ID = uuid.New().String()
	h.store.SaveQuiz(quiz)

	http.Redirect(w, r, "/host/quiz/"+quiz.ID, http.StatusFound)
}

func (h *Handler) EditQuiz(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	quiz, _ := h.store.GetQuiz(id)
	if quiz == nil {
		http.Redirect(w, r, "/host", http.StatusFound)
		return
	}
	h.templates.ExecuteTemplate(w, "templates/host/editor.html", map[string]interface{}{
		"Quiz": quiz,
	})
}

func (h *Handler) SaveQuiz(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	quiz, _ := h.store.GetQuiz(id)
	if quiz == nil {
		http.Redirect(w, r, "/host", http.StatusFound)
		return
	}

	quiz.Title = r.FormValue("title")
	quiz.Description = r.FormValue("description")

	questionCount, _ := strconv.Atoi(r.FormValue("question_count"))
	quiz.Questions = nil

	for i := 0; i < questionCount; i++ {
		qText := r.FormValue("question_text_" + strconv.Itoa(i))
		if qText == "" {
			continue
		}

		correctIdx, _ := strconv.Atoi(r.FormValue("correct_" + strconv.Itoa(i)))

		answers := []string{}
		for j := 0; j < 6; j++ {
			if ans := r.FormValue("answer_" + strconv.Itoa(i) + "_" + strconv.Itoa(j)); ans != "" {
				answers = append(answers, ans)
			}
		}

		if len(answers) >= 2 {
			qID := r.FormValue("question_id_" + strconv.Itoa(i))
			if qID == "" {
				qID = uuid.New().String()
			}
			quiz.Questions = append(quiz.Questions, models.Question{
				ID:           qID,
				Text:         qText,
				Answers:      answers,
				CorrectIndex: correctIdx,
			})
		}
	}

	h.store.SaveQuiz(quiz)
	http.Redirect(w, r, "/host/quiz/"+id, http.StatusFound)
}

func (h *Handler) DeleteQuiz(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.store.DeleteQuiz(id)
	http.Redirect(w, r, "/host", http.StatusFound)
}

func (h *Handler) StartSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	quiz, _ := h.store.GetQuiz(id)
	if quiz == nil {
		http.Redirect(w, r, "/host", http.StatusFound)
		return
	}

	timer, _ := strconv.Atoi(r.FormValue("timer"))
	shuffle := r.FormValue("shuffle") == "on"

	roomCode := generateRoomCode()
	hostToken := generateToken()

	session := models.NewSession(id, roomCode, hostToken, timer, shuffle)
	session.ID = uuid.New().String()
	h.store.CreateSession(session)

	http.SetCookie(w, &http.Cookie{
		Name:  "host_token",
		Value: hostToken,
		Path:  "/",
	})
	http.Redirect(w, r, "/host/session/"+roomCode, http.StatusFound)
}

func (h *Handler) HostSession(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Redirect(w, r, "/host", http.StatusFound)
		return
	}

	quiz, _ := h.store.GetQuiz(session.QuizID)
	data := h.viewDataForSession(session, quiz)
	h.templates.ExecuteTemplate(w, "templates/host/session.html", data)
}

func (h *Handler) NextQuestion(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	quiz, _ := h.store.GetQuiz(session.QuizID)
	if quiz == nil {
		http.Error(w, "Quiz not found", http.StatusNotFound)
		return
	}

	session.CurrentIndex++
	if session.CurrentIndex >= len(quiz.Questions) {
		session.CurrentIndex = len(quiz.Questions) - 1
	}

	session.Status = models.StatusActive
	session.Answers = make(map[string]*models.Answer)
	h.store.UpdateSession(session)

	question := quiz.Questions[session.CurrentIndex]
	h.broker.Broadcast(code, "question_start", map[string]interface{}{
		"index":   session.CurrentIndex,
		"text":    question.Text,
		"answers": question.Answers,
		"timer":   session.TimerSeconds,
	})

	if session.TimerSeconds > 0 {
		h.timerMgr.Start(code, session.TimerSeconds)
	}

	http.Redirect(w, r, "/host/session/"+code, http.StatusSeeOther)
}

func (h *Handler) EndRound(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	quiz, _ := h.store.GetQuiz(session.QuizID)
	if quiz == nil {
		http.Error(w, "Quiz not found", http.StatusNotFound)
		return
	}

	h.timerMgr.Cancel(code)

	session.Status = models.StatusReveal
	h.store.UpdateSession(session)

	question := quiz.Questions[session.CurrentIndex]
	stats := make([]int, len(question.Answers))
	for _, ans := range session.Answers {
		if ans.QuestionIndex == session.CurrentIndex {
			stats[ans.SelectedIndex]++
		}
	}

	h.broker.Broadcast(code, "question_end", map[string]interface{}{
		"correct_index": question.CorrectIndex,
		"stats":         stats,
	})

	http.Redirect(w, r, "/host/session/"+code, http.StatusSeeOther)
}

func (h *Handler) RevealAnswer(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	quiz, _ := h.store.GetQuiz(session.QuizID)
	if quiz == nil || session.CurrentIndex < 0 || session.CurrentIndex >= len(quiz.Questions) {
		http.Error(w, "No question to reveal", http.StatusNotFound)
		return
	}

	h.timerMgr.Cancel(code)

	question := quiz.Questions[session.CurrentIndex]
	stats := make([]int, len(question.Answers))
	for _, ans := range session.Answers {
		if ans.QuestionIndex == session.CurrentIndex {
			stats[ans.SelectedIndex]++
		}
	}

	session.Status = models.StatusReveal
	h.store.UpdateSession(session)

	h.broker.Broadcast(code, "answer_reveal", map[string]interface{}{
		"correct_index": question.CorrectIndex,
		"stats":         stats,
	})

	http.Redirect(w, r, "/host/session/"+code, http.StatusSeeOther)
}

func (h *Handler) EndSession(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	now := time.Now()
	session.Status = models.StatusEnded
	session.EndedAt = &now
	h.store.UpdateSession(session)

	h.broker.Broadcast(code, "session_ended", map[string]interface{}{
		"rankings": h.getLeaderboard(session),
	})

	http.Redirect(w, r, "/host", http.StatusSeeOther)
}

func (h *Handler) JoinRedirect(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(r.URL.Query().Get("code"))
	if code == "" {
		http.Redirect(w, r, "/?error=nocode", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/join/"+code, http.StatusFound)
}

func (h *Handler) JoinPage(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	errorMsg := r.URL.Query().Get("error")

	var errorText string
	switch errorMsg {
	case "notfound":
		errorText = "Session not found"
	case "noname":
		errorText = "Please enter a nickname"
	}

	h.templates.ExecuteTemplate(w, "templates/play/join.html", map[string]interface{}{
		"Code":    code,
		"Session": session,
		"Error":   errorText,
	})
}

func (h *Handler) JoinSession(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	log.Printf("[DEBUG] JoinSession: code=%s", code)

	session, err := h.sessionService.GetSession(code)
	if err != nil || session == nil {
		log.Printf("[DEBUG] JoinSession: session not found, code=%s", code)
		http.Redirect(w, r, "/join/"+code+"?error=notfound", http.StatusFound)
		return
	}

	nickname := strings.TrimSpace(r.FormValue("nickname"))
	log.Printf("[DEBUG] JoinSession: nickname=%s, session status=%s", nickname, session.Status)
	if nickname == "" {
		http.Redirect(w, r, "/join/"+code+"?error=noname", http.StatusFound)
		return
	}

	session, player, err := h.sessionService.JoinSession(code, nickname)
	if err != nil {
		log.Printf("[DEBUG] JoinSession: error joining: %v", err)
		http.Redirect(w, r, "/join/"+code+"?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}

	log.Printf("[DEBUG] JoinSession: player joined, id=%s, nickname=%s, total players=%d", player.ID, player.Nickname, len(session.Players))

	// Broadcast player joined event with nickname
	h.broker.Broadcast(code, "player_joined", map[string]interface{}{
		"nickname": player.Nickname,
		"count":    len(session.Players),
	})

	http.SetCookie(w, &http.Cookie{
		Name:  "player_id",
		Value: player.ID,
		Path:  "/",
		// Add SameSite for better cookie handling
		SameSite: http.SameSiteLaxMode,
	})
	log.Printf("[DEBUG] JoinSession: redirecting to /play/%s", code)
	http.Redirect(w, r, "/play/"+code, http.StatusFound)
}

func (h *Handler) PlaySession(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	log.Printf("[DEBUG] PlaySession: code=%s", code)

	session, err := h.sessionService.GetSession(code)
	if err != nil || session == nil {
		log.Printf("[DEBUG] PlaySession: session not found, code=%s", code)
		http.Redirect(w, r, "/join/"+code+"?error=notfound", http.StatusFound)
		return
	}

	log.Printf("[DEBUG] PlaySession: session status=%s, players=%d", session.Status, len(session.Players))

	quiz, err := h.quizService.GetQuiz(session.QuizID)
	if err != nil || quiz == nil {
		log.Printf("[DEBUG] PlaySession: quiz not found, quizID=%s", session.QuizID)
		http.Redirect(w, r, "/join/"+code+"?error=quiznotfound", http.StatusFound)
		return
	}

	playerID := ""
	if cookie, err := r.Cookie("player_id"); err == nil {
		playerID = cookie.Value
		log.Printf("[DEBUG] PlaySession: cookie player_id=%s", playerID)
	} else {
		log.Printf("[DEBUG] PlaySession: no player_id cookie, err=%v", err)
	}

	// Check if player exists in session
	player, exists := session.Players[playerID]
	if !exists {
		log.Printf("[DEBUG] PlaySession: player not found in session, playerID=%s, available players=%v", playerID, getPlayerIDs(session.Players))
		// Player not found, redirect to join
		http.Redirect(w, r, "/join/"+code, http.StatusFound)
		return
	}

	log.Printf("[DEBUG] PlaySession: player found, nickname=%s", player.Nickname)

	data := map[string]interface{}{
		"Code":     code,
		"Session":  session,
		"Quiz":     quiz,
		"Player":   player,
		"PlayerID": playerID,
		"Title":    quiz.Title,
	}
	h.templates.ExecuteTemplate(w, "templates/play/session.html", data)
}

func getPlayerIDs(players map[string]*models.Player) []string {
	ids := make([]string, 0, len(players))
	for id := range players {
		ids = append(ids, id)
	}
	return ids
}

func (h *Handler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	cookie, err := r.Cookie("player_id")
	if err != nil {
		http.Error(w, "Not joined", http.StatusUnauthorized)
		return
	}
	playerID := cookie.Value

	if _, ok := session.Answers[playerID]; ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": false, "error": "already answered"}`))
		return
	}

	answerIdx, _ := strconv.Atoi(r.FormValue("answer"))
	quiz, _ := h.store.GetQuiz(session.QuizID)

	var responseTime int64
	if startTime := r.FormValue("start_time"); startTime != "" {
		if start, err := strconv.ParseInt(startTime, 10, 64); err == nil {
			responseTime = time.Now().UnixMilli() - start
		}
	}

	answer := &models.Answer{
		PlayerID:      playerID,
		QuestionIndex: session.CurrentIndex,
		SelectedIndex: answerIdx,
		ResponseTime:  responseTime,
	}

	if session.CurrentIndex < len(quiz.Questions) {
		if answerIdx == quiz.Questions[session.CurrentIndex].CorrectIndex {
			if player, ok := session.Players[playerID]; ok {
				player.Score++
				player.TotalTime += responseTime
				player.Answers[session.CurrentIndex] = true
			}
		}
	}

	session.Answers[playerID] = answer
	h.store.UpdateSession(session)

	h.broker.Broadcast(code, "answer_submitted", map[string]interface{}{
		"player_id": playerID,
		"count":     len(session.Answers),
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success": true}`))
}

func (h *Handler) SSEHandler(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	h.broker.ServeHTTP(w, r, code)
}

func (h *Handler) APISession(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (h *Handler) APILeaderboard(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.getLeaderboard(session))
}

func (h *Handler) APIStats(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	quiz, _ := h.store.GetQuiz(session.QuizID)
	if quiz == nil || session.CurrentIndex < 0 || session.CurrentIndex >= len(quiz.Questions) {
		http.Error(w, "No stats available", http.StatusNotFound)
		return
	}

	question := quiz.Questions[session.CurrentIndex]
	stats := make([]int, len(question.Answers))
	for _, ans := range session.Answers {
		if ans.QuestionIndex == session.CurrentIndex {
			stats[ans.SelectedIndex]++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) getLeaderboard(session *models.Session) []map[string]interface{} {
	type rankedPlayer struct {
		player   *models.Player
		playerID string
		avgTime  int64
	}

	players := make([]rankedPlayer, 0, len(session.Players))
	for pid, p := range session.Players {
		avgTime := int64(0)
		if len(p.Answers) > 0 {
			avgTime = p.TotalTime / int64(len(p.Answers))
		}
		players = append(players, rankedPlayer{player: p, playerID: pid, avgTime: avgTime})
	}

	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			if players[j].player.Score > players[i].player.Score ||
				(players[j].player.Score == players[i].player.Score && players[j].avgTime < players[i].avgTime) {
				players[i], players[j] = players[j], players[i]
			}
		}
	}

	result := make([]map[string]interface{}, len(players))
	for i, rp := range players {
		result[i] = map[string]interface{}{
			"rank":      i + 1,
			"player_id": rp.playerID,
			"nickname":  rp.player.Nickname,
			"score":     rp.player.Score,
			"avg_time":  rp.avgTime,
		}
	}
	return result
}

// viewDataForSession builds template data with per-question stats for reveal UI.
func (h *Handler) viewDataForSession(session *models.Session, quiz *models.Quiz) map[string]interface{} {
	d := map[string]interface{}{
		"Session": session,
		"Quiz":    quiz,
	}
	if session != nil {
		d["Rankings"] = h.getLeaderboard(session)
	} else {
		d["Rankings"] = []map[string]interface{}{}
	}
	if session == nil || quiz == nil {
		d["CorrectIndex"] = -1
		return d
	}
	d["CorrectIndex"] = -1
	if session.CurrentIndex < 0 || session.CurrentIndex >= len(quiz.Questions) {
		return d
	}
	q := quiz.Questions[session.CurrentIndex]
	stats := make([]int, len(q.Answers))
	total := 0
	for _, ans := range session.Answers {
		if ans.QuestionIndex == session.CurrentIndex {
			stats[ans.SelectedIndex]++
			total++
		}
	}
	barWidths := make([]int, len(stats))
	for i, c := range stats {
		if total > 0 {
			barWidths[i] = c * 100 / total
		}
	}
	d["Stats"] = stats
	d["CorrectIndex"] = q.CorrectIndex
	d["StatsTotal"] = total
	d["BarWidths"] = barWidths
	return d
}

func (h *Handler) HostUISync(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	quiz, _ := h.store.GetQuiz(session.QuizID)
	data := h.viewDataForSession(session, quiz)

	var mainBuf, controlsBuf, playersBuf, lbBuf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&mainBuf, "templates/partials/host_main.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.templates.ExecuteTemplate(&controlsBuf, "templates/partials/host_controls.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.templates.ExecuteTemplate(&playersBuf, "templates/partials/players_panel.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.templates.ExecuteTemplate(&lbBuf, "templates/partials/leaderboard.html", map[string]interface{}{
		"Rankings": h.getLeaderboard(session),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"main":        mainBuf.String(),
		"controls":    controlsBuf.String(),
		"players":     playersBuf.String(),
		"leaderboard": lbBuf.String(),
	})
}

func (h *Handler) PlayUISync(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	quiz, _ := h.store.GetQuiz(session.QuizID)
	playerID := ""
	if cookie, err := r.Cookie("player_id"); err == nil {
		playerID = cookie.Value
	}
	data := h.viewDataForSession(session, quiz)
	data["Code"] = code
	data["PlayerID"] = playerID

	var mainBuf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&mainBuf, "templates/partials/play_main.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"main": mainBuf.String(),
	})
}

func (h *Handler) PartialQuestion(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		return
	}

	quiz, _ := h.store.GetQuiz(session.QuizID)
	if quiz == nil || session.CurrentIndex < 0 || session.CurrentIndex >= len(quiz.Questions) {
		return
	}

	question := quiz.Questions[session.CurrentIndex]
	h.templates.ExecuteTemplate(w, "templates/partials/question.html", map[string]interface{}{
		"Question": question,
		"Index":    session.CurrentIndex,
	})
}

func (h *Handler) PartialLeaderboard(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		return
	}

	h.templates.ExecuteTemplate(w, "templates/partials/leaderboard.html", map[string]interface{}{
		"Rankings": h.getLeaderboard(session),
	})
}

func (h *Handler) PartialReveal(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, _ := h.store.GetSession(code)
	if session == nil {
		return
	}

	quiz, _ := h.store.GetQuiz(session.QuizID)
	if quiz == nil || session.CurrentIndex < 0 || session.CurrentIndex >= len(quiz.Questions) {
		return
	}

	data := h.viewDataForSession(session, quiz)
	h.templates.ExecuteTemplate(w, "templates/partials/reveal.html", data)
}

func (h *Handler) timerEndRound(roomCode string) {
	session, _ := h.store.GetSession(roomCode)
	if session == nil || session.Status != models.StatusActive {
		return
	}

	quiz, _ := h.store.GetQuiz(session.QuizID)
	if quiz == nil {
		return
	}

	session.Status = models.StatusReveal
	h.store.UpdateSession(session)

	question := quiz.Questions[session.CurrentIndex]
	stats := make([]int, len(question.Answers))
	for _, ans := range session.Answers {
		if ans.QuestionIndex == session.CurrentIndex {
			stats[ans.SelectedIndex]++
		}
	}

	h.broker.Broadcast(roomCode, "question_end", map[string]interface{}{
		"correct_index": question.CorrectIndex,
		"stats":         stats,
	})
}

// API endpoints

func (h *Handler) APISessionInfo(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, err := h.sessionService.GetSession(code)
	if err != nil || session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (h *Handler) SSEEvents(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	session, err := h.sessionService.GetSession(code)
	if err != nil || session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Use the existing SSE broker implementation
	h.broker.ServeHTTP(w, r, code)
}
