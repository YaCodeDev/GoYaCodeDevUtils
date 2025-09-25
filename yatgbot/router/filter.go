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

type Filter func(ctx context.Context, deps FilterDependencies) (bool, yaerrors.Error)

type FilterDependencies struct {
	storage yafsm.EntityFSMStorage
	userID  int64
	update  tg.UpdateClass
}

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

func TextEq(want string) Filter {
	return func(_ context.Context, deps FilterDependencies) (bool, yaerrors.Error) {
		if m, ok := extractMessageFromUpdate(deps.update); ok && m.Message == want {
			return true, nil
		}

		return false, nil
	}
}

func TextRegex(re *regexp.Regexp) Filter {
	return func(_ context.Context, deps FilterDependencies) (bool, yaerrors.Error) {
		if m, ok := extractMessageFromUpdate(deps.update); ok && re.MatchString(m.Message) {
			return true, nil
		}

		return false, nil
	}
}

func CallbackEq(data string) Filter {
	return func(_ context.Context, deps FilterDependencies) (bool, yaerrors.Error) {
		if q, ok := deps.update.(*tg.UpdateBotCallbackQuery); ok && string(q.Data) == data {
			return true, nil
		}

		return false, nil
	}
}

func CallbackPrefix(prefix string) Filter {
	return func(_ context.Context, deps FilterDependencies) (bool, yaerrors.Error) {
		if q, ok := deps.update.(*tg.UpdateBotCallbackQuery); ok &&
			strings.HasPrefix(string(q.Data), prefix) {
			return true, nil
		}

		return false, nil
	}
}
