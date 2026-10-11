package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"roomly-api/internal/models"
)

func openIsolatedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("ROOMLY_TEST_DSN")
	if dsn == "" {
		t.Skip("ROOMLY_TEST_DSN is not configured; MySQL integration tests were skipped")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("ROOMLY_TEST_DSN is invalid")
	}
	dbName := strings.ToLower(cfg.DBName)
	if !(strings.HasSuffix(dbName, "_test") || strings.HasSuffix(dbName, "_testing")) {
		t.Fatal("refusing to run integration tests unless the database name ends in _test or _testing")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("could not open isolated MySQL test database")
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal("could not connect to isolated MySQL test database")
	}
	return db
}

func createBookingFixture(t *testing.T, db *sql.DB) (int64, int64, func()) {
	t.Helper()
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	userResult, err := db.ExecContext(ctx, `INSERT INTO users (name, email, password_hash, role) VALUES (?, ?, ?, 'employee')`, "Integration Test", fmt.Sprintf("booking-%x@example.invalid", random), "not-a-real-password-hash")
	if err != nil {
		t.Fatalf("create isolated test user: %v", err)
	}
	userID, err := userResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	roomResult, err := db.ExecContext(ctx, `INSERT INTO rooms (name, capacity, location) VALUES (?, ?, ?)`, fmt.Sprintf("Integration %x", random), 20, "Test only")
	if err != nil {
		t.Fatalf("create isolated test room: %v", err)
	}
	roomID, err := roomResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_, _ = db.ExecContext(cleanCtx, `DELETE FROM bookings WHERE user_id = ?`, userID)
		_, _ = db.ExecContext(cleanCtx, `DELETE FROM users WHERE id = ?`, userID)
		_, _ = db.ExecContext(cleanCtx, `DELETE FROM rooms WHERE id = ?`, roomID)
	}
	t.Cleanup(cleanup)
	return roomID, userID, cleanup
}

func TestBookingCreateSerializesOverlappingRequests(t *testing.T) {
	db := openIsolatedTestDB(t)
	roomID, userID, _ := createBookingFixture(t, db)
	repo := NewBookingRepository(db)
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	booking := models.Booking{RoomID: roomID, UserID: userID, Title: "Concurrent Test", AttendeeCount: 2, StartTime: start, EndTime: start.Add(time.Hour)}
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := repo.Create(context.Background(), booking); errs <- err }()
	}
	wg.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, ErrBookingConflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent booking error: %v", err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want one of each", successes, conflicts)
	}
	backToBack := booking
	backToBack.Title = "Back-to-back Test"
	backToBack.StartTime = booking.EndTime
	backToBack.EndTime = booking.EndTime.Add(time.Hour)
	if _, err := repo.Create(context.Background(), backToBack); err != nil {
		t.Fatalf("back-to-back booking should be allowed: %v", err)
	}
}

func TestBookingRejectsCapacityAndInactiveRoom(t *testing.T) {
	db := openIsolatedTestDB(t)
	roomID, userID, _ := createBookingFixture(t, db)
	repo := NewBookingRepository(db)
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	booking := models.Booking{RoomID: roomID, UserID: userID, Title: "Capacity Test", AttendeeCount: 21, StartTime: start, EndTime: start.Add(time.Hour)}
	if _, err := repo.Create(context.Background(), booking); !errors.Is(err, ErrBookingCapacity) {
		t.Fatalf("capacity error = %v, want %v", err, ErrBookingCapacity)
	}
	if _, err := db.Exec(`UPDATE rooms SET is_active = FALSE WHERE id = ?`, roomID); err != nil {
		t.Fatal("deactivate test room")
	}
	booking.AttendeeCount = 1
	if _, err := repo.Create(context.Background(), booking); !errors.Is(err, ErrRoomInactive) {
		t.Fatalf("inactive room error = %v, want %v", err, ErrRoomInactive)
	}
}

func TestCancelledBookingDoesNotBlockAvailability(t *testing.T) {
	db := openIsolatedTestDB(t)
	roomID, userID, _ := createBookingFixture(t, db)
	repo := NewBookingRepository(db)
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	b, err := repo.Create(context.Background(), models.Booking{RoomID: roomID, UserID: userID, Title: "Cancel Test", AttendeeCount: 1, StartTime: start, EndTime: start.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Cancel(context.Background(), b.ID); err != nil {
		t.Fatal(err)
	}
	available, err := repo.CheckAvailability(context.Background(), roomID, start, start.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Fatal("cancelled booking still blocks availability")
	}
}

func TestFirstAdminBootstrapAndLastAdminGuard(t *testing.T) {
	db := openIsolatedTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var existingAdmins int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&existingAdmins); err != nil {
		t.Fatal("check isolated test admins")
	}
	if existingAdmins != 0 {
		t.Skip("isolated test database already has an admin; refusing to change its role state")
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	email := fmt.Sprintf("seed-%x@example.invalid", random)
	result, err := db.ExecContext(ctx, `INSERT INTO users (name, email, password_hash, role) VALUES (?, ?, ?, 'employee')`, "Bootstrap Test", email, "not-a-real-password-hash")
	if err != nil {
		t.Fatal("create isolated bootstrap user")
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM users WHERE id = ?`, userID)
	})
	repo := NewUserRepository(db)
	if err := repo.PromoteFirstAdmin(ctx, email); err != nil {
		t.Fatal("bootstrap first admin")
	}
	user, err := repo.FindByID(ctx, userID)
	if err != nil || user.Role != "admin" {
		t.Fatalf("bootstrapped user = %#v, %v", user, err)
	}
	if _, err := repo.SetActive(ctx, userID, false); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("deactivate last admin error = %v", err)
	}
	if _, err := repo.SetRole(ctx, userID, "employee"); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("demote last admin error = %v", err)
	}
}

func TestAmenityCRUDAndRoomAssociation(t *testing.T) {
	db := openIsolatedTestDB(t)
	roomID, _, _ := createBookingFixture(t, db)
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("Test Amenity %x", random)
	repo := NewAmenityRepository(db)
	a, err := repo.Create(context.Background(), models.Amenity{Name: name})
	if err != nil {
		t.Fatal("create amenity")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.ExecContext(ctx, `DELETE FROM room_amenities WHERE room_id = ?`, roomID)
		_, _ = db.ExecContext(ctx, `DELETE FROM amenities WHERE id = ?`, a.ID)
	})
	if _, err := repo.Create(context.Background(), models.Amenity{Name: name}); !errors.Is(err, ErrAmenityExists) {
		t.Fatalf("duplicate amenity error = %v", err)
	}
	if err := repo.AddToRoom(context.Background(), roomID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddToRoom(context.Background(), roomID, a.ID); !errors.Is(err, ErrAssociationExists) {
		t.Fatalf("duplicate association error = %v", err)
	}
	items, err := repo.ListForRoom(context.Background(), roomID)
	if err != nil || len(items) != 1 || items[0].ID != a.ID {
		t.Fatalf("room amenities = %#v, %v", items, err)
	}
	if err := repo.RemoveFromRoom(context.Background(), roomID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
}
