# No persistence: in-memory rolling window only

The monitor keeps only a few minutes of samples in an in-memory rolling buffer (enough to draw the live throughput curve) and writes nothing to disk. "Query" means filtering, sorting, and top-N over the *current* activity, not time-travel over history.

This is a deliberate privacy choice: persisting samples would mean logging every remote endpoint the machine ever contacts — a sensitive record to accumulate for a tool a user runs casually. It also removes a whole class of concerns (database lifecycle, disk growth, cleanup). If historical querying is ever wanted, it should be additive and opt-in behind an explicit flag, never the default.
