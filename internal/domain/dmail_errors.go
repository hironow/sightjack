package domain

import "errors"

var (
	ErrDMailSchemaRequired      = errors.New("dmail: dmail-schema-version is required")
	ErrDMailSchemaUnsupported   = errors.New("dmail: unsupported dmail-schema-version")
	ErrDMailNameRequired        = errors.New("dmail: name is required")
	ErrDMailKindRequired        = errors.New("dmail: kind is required")
	ErrDMailKindInvalid         = errors.New("dmail: invalid kind (not in canonical set)")
	ErrDMailDescriptionRequired = errors.New("dmail: description is required")
	ErrDMailActionInvalid       = errors.New("dmail: invalid action")
)
