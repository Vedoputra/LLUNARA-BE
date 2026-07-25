package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeAccountRepo struct {
	deleteCalledWith uuid.UUID
	deleteErr        error
}

func (f *fakeAccountRepo) DeleteAllUserData(_ context.Context, userID uuid.UUID) error {
	f.deleteCalledWith = userID
	return f.deleteErr
}

type fakeAuthAdminClient struct {
	deleteCalled bool
	deleteErr    error
}

func (f *fakeAuthAdminClient) DeleteUser(context.Context, uuid.UUID) error {
	f.deleteCalled = true
	return f.deleteErr
}

func TestDeleteAccount_DeletesDataThenAuthUser(t *testing.T) {
	repo := &fakeAccountRepo{}
	authClient := &fakeAuthAdminClient{}
	svc := NewAccountService(repo, authClient)

	if err := svc.DeleteAccount(context.Background(), testUUID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if repo.deleteCalledWith != testUUID {
		t.Errorf("repo.DeleteAllUserData called with %v, want %v", repo.deleteCalledWith, testUUID)
	}
	if !authClient.deleteCalled {
		t.Error("expected authClient.DeleteUser to be called")
	}
}

func TestDeleteAccount_StopsBeforeAuthDeleteIfDataDeleteFails(t *testing.T) {
	repo := &fakeAccountRepo{deleteErr: errors.New("db down")}
	authClient := &fakeAuthAdminClient{}
	svc := NewAccountService(repo, authClient)

	if err := svc.DeleteAccount(context.Background(), testUUID); err == nil {
		t.Fatal("expected an error when data deletion fails")
	}
	if authClient.deleteCalled {
		t.Error("expected authClient.DeleteUser NOT to be called when app data deletion failed")
	}
}

func TestDeleteAccount_ReturnsErrorIfAuthDeleteFails(t *testing.T) {
	repo := &fakeAccountRepo{}
	authClient := &fakeAuthAdminClient{deleteErr: errors.New("bad_jwt")}
	svc := NewAccountService(repo, authClient)

	if err := svc.DeleteAccount(context.Background(), testUUID); err == nil {
		t.Fatal("expected an error when auth user deletion fails")
	}
	// App data deletion must still have run and succeeded — that's the part
	// that matters most for a health-data app, even if the auth cleanup
	// needs a retry.
	if repo.deleteCalledWith != testUUID {
		t.Error("expected app data to be deleted even though auth deletion later failed")
	}
}
