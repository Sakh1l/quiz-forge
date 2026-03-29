package router

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/quizforge/quiz-forge/internal/config"
	"github.com/quizforge/quiz-forge/internal/handler"
	"github.com/quizforge/quiz-forge/internal/middleware"
	"github.com/quizforge/quiz-forge/internal/repository/memory"
	"github.com/quizforge/quiz-forge/internal/repository/sqlite"
	"github.com/quizforge/quiz-forge/internal/sse"
)

func syncFromSQLite(mem *memory.MemoryStore, sql *sqlite.SQLiteStore) error {
	quizzes, err := sql.ListQuizzes()
	if err != nil {
		return err
	}
	for _, q := range quizzes {
		mem.SaveQuiz(q)
	}

	sessions, err := sql.ListActiveSessions()
	if err != nil {
		return err
	}
	for _, s := range sessions {
		mem.CreateSession(s)
	}
	return nil
}

func syncToSQLiteLoop(mem *memory.MemoryStore, sql *sqlite.SQLiteStore) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sessions := mem.ListActiveSessions()
		for _, s := range sessions {
			sql.UpdateSession(s)
		}

		quizzes := mem.ListQuizzes()
		for _, q := range quizzes {
			sql.SaveQuiz(q)
		}
	}
}

func New(cfg *config.Config) *chi.Mux {
	store := memory.NewMemoryStore()

	if cfg.IsProduction() {
		sqliteStore, err := sqlite.NewSQLiteStore(cfg.DatabasePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize SQLite store: %v, falling back to memory only\n", err)
		} else {
			if err := syncFromSQLite(store, sqliteStore); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to sync from SQLite: %v\n", err)
			}
			if err := sqliteStore.InitSampleQuiz(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to initialize sample quiz: %v\n", err)
			}
			go syncToSQLiteLoop(store, sqliteStore)
		}
	}

	broker := sse.NewBroker()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.CORS(cfg))
	r.Use(middleware.Logger(cfg))
	r.Use(middleware.Recoverer(cfg))
	r.Use(middleware.RateLimit(cfg))

	r.Handle("/static/*", http.FileServer(http.Dir("static")))

	h := handler.NewHandler(store, broker, cfg)

	r.Get("/", h.Index)
	r.Get("/host", h.HostDashboard)
	r.Post("/host/quiz/create", h.CreateQuiz)
	r.Get("/host/quiz/{id}", h.EditQuiz)
	r.Post("/host/quiz/{id}", h.SaveQuiz)
	r.Post("/host/quiz/{id}/delete", h.DeleteQuiz)
	r.Post("/host/quiz/{id}/start", h.StartSession)

	r.Post("/host/session/{code}/next", h.NextQuestion)
	r.Post("/host/session/{code}/end-round", h.EndRound)
	r.Post("/host/session/{code}/reveal", h.RevealAnswer)
	r.Post("/host/session/{code}/end", h.EndSession)
	r.Get("/host/session/{code}", h.HostSession)

	r.Get("/join", h.JoinRedirect)
	r.Get("/join/{code}", h.JoinPage)
	r.Post("/join/{code}", h.JoinSession)
	r.Get("/play/{code}", h.PlaySession)
	r.Post("/play/{code}/answer", h.SubmitAnswer)

	r.Get("/events/{code}", h.SSEHandler)

	r.Get("/partials/question/{code}", h.PartialQuestion)
	r.Get("/partials/leaderboard/{code}", h.PartialLeaderboard)
	r.Get("/partials/reveal/{code}", h.PartialReveal)

	r.Get("/api/session/{code}", h.APISessionInfo)
	r.Get("/api/session/{code}/leaderboard", h.APILeaderboard)
	r.Get("/api/session/{code}/stats", h.APIStats)
	r.Get("/api/session/{code}/events", h.SSEEvents)

	r.Get("/api/ui/host/{code}", h.HostUISync)
	r.Get("/api/ui/play/{code}", h.PlayUISync)

	return r
}
