# Examples

These programs demonstrate the three TypeSafe primitives using the scenarios
from the official primitive documentation:

- [`choice`](choice) routes a support ticket to one team and reads the selected
  option, confidence, and probability distribution.
- [`score`](score) rates a bug against ordered severity levels and keeps the
  returned score fractional.
- [`noul`](noul) evaluates two yes/no questions, including one with explicit
  true/false criteria, and applies an application-owned threshold.

Set an API key, then run any example from the module root:

```sh
export TYPESAFE_API_KEY="your-api-key"
go run ./examples/choice
go run ./examples/score
go run ./examples/noul
```

These commands call the live TypeSafe API and may incur usage. Tests in this
repository do not run them and remain offline.
