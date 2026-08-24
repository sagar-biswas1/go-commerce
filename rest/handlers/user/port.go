package user

import (
	"context"

	"go-commerce/domain"

	"github.com/google/uuid"
)

// Service is the application service this handler drives, declared here by the
// consumer that uses it.
//
// It names only domain types, which is what keeps this file honest: the handler
// depends on the vocabulary of the business and on no package that sits beside
// or below it. The real user service satisfies this because Go matches
// interfaces by shape, not by name -- so nothing has to import the handler, and
// a test can drive these routes with a fake instead of a database.
//
// Update and ChangePassword take the actor rather than reading it from the
// context themselves. Authorization for a user depends on what is being changed,
// not only on who is asking, so the decision belongs in the service -- and a
// service that is handed the actor cannot forget to look for one.
type Service interface {
	List(ctx context.Context, page domain.Page, filter *domain.UserFilter) (*domain.PageResult[*domain.User], error)
	Get(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Create(ctx context.Context, input *domain.UserCreateInput) (*domain.User, error)
	Update(ctx context.Context, actor domain.Identity, id uuid.UUID, patch *domain.UserPatch) (*domain.User, error)
	ChangePassword(ctx context.Context, actor domain.Identity, id uuid.UUID, current, next string) error
	Delete(ctx context.Context, id uuid.UUID) error
}
