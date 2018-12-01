package analyzesqueue

import "time"

const VisibilityTimeoutSec = 600          // must be in sync with cloudformation.yml
const ConsumerTimeout = 530 * time.Second // reserve 30 sec
