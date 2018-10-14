package primaryqueue

import "time"

const VisibilityTimeoutSec = 60          // must be in sync with cloudformation.yml
const ConsumerTimeout = 45 * time.Second // reserve 15 sec
