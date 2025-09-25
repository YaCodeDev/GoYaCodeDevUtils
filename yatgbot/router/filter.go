package router

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/gotd/td/tg"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yafsm"
)

// Filter is a function that determines whether a given update should be processed
type Filter func(ctx context.Context, deps FilterDependencies) (bool, yaerrors.Error)

// FilterDependencies holds the dependencies required by filters
type FilterDependencies struct {
	storage yafsm.EntityFSMStorage
	userID  int64
	update  tg.UpdateClass
}

// StateIs creates a filter that checks if the user's state matches any of the provided states.
func StateIs(want ...string) Filter {
	wanted := make(map[string]struct{}, len(want))

	for _, s := range want {
		wanted[s] = struct{}{}
	}

	return func(ctx context.Context, deps FilterDependencies) (bool, yaerrors.Error) {
		state, _, err := deps.storage.GetState(ctx)
		if err != nil {
			return false, yaerrors.FromError(
				http.StatusInternalServerError,
				err, "failed to get state for user %d",
			)
		}

		_, ok := wanted[state]

		return ok, nil
	}
}

// TextEq creates a filter that checks if the message text equals the specified string.
func TextEq(want string) Filter {
	return func(_ context.Context, deps FilterDependencies) (bool, yaerrors.Error) {
		if m, ok := extractMessageFromUpdate(deps.update); ok && m.Message == want {
			return true, nil
		}

		return false, nil
	}
}

// TextRegex creates a filter that checks if the message text matches the specified regex.
func TextRegex(re *regexp.Regexp) Filter {
	return func(_ context.Context, deps FilterDependencies) (bool, yaerrors.Error) {
		if m, ok := extractMessageFromUpdate(deps.update); ok && re.MatchString(m.Message) {
			return true, nil
		}

		return false, nil
	}
}

// CallbackEq creates a filter that checks if the callback query data equals the specified string.
func CallbackEq(data string) Filter {
	return func(_ context.Context, deps FilterDependencies) (bool, yaerrors.Error) {
		if q, ok := deps.update.(*tg.UpdateBotCallbackQuery); ok && string(q.Data) == data {
			return true, nil
		}

		return false, nil
	}
}

// CallbackPrefix creates a filter that checks if the callback query data starts with the specified prefix.
func CallbackPrefix(prefix string) Filter {
	return func(_ context.Context, deps FilterDependencies) (bool, yaerrors.Error) {
		if q, ok := deps.update.(*tg.UpdateBotCallbackQuery); ok &&
			strings.HasPrefix(string(q.Data), prefix) {
			return true, nil
		}

		return false, nil
	}
}
