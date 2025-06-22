import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Counter } from "k6/metrics";

const errorRate = new Rate("errors");

// Counters for common error statuses
const errorCounters = {
  clientErrors: new Counter("client_errors"), // 4xx errors
  serverErrors: new Counter("server_errors"), // 5xx errors
  timeoutErrors: new Counter("timeout_errors"), // Request timeouts
  validationErrors: new Counter("validation_errors"), // 400 Bad Request
  rateLimitErrors: new Counter("rate_limit_errors"), // 429 Too Many Requests
  internalServerErrors: new Counter("internal_server_errors"), // 500 Internal Server Error
  serviceUnavailableErrors: new Counter("service_unavailable_errors"), // 503 Service Unavailable
};

// Map status codes to their corresponding counters
const statusCodeToCounter = new Map([
  [400, errorCounters.validationErrors],
  [429, errorCounters.rateLimitErrors],
  [500, errorCounters.internalServerErrors],
  [503, errorCounters.serviceUnavailableErrors],
]);

const only201Status = http.expectedStatuses(201);

export const options = {
  scenarios: {
    // Linear load test - gradually increase from 0 to 1000 RPS over 5 minutes
    linear_load: {
      executor: "ramping-arrival-rate",
      startRate: 0,
      timeUnit: "1s",
      preAllocatedVUs: 100,
      maxVUs: 1000,
      stages: [
        { duration: "5s", target: 200 }, // Ramp up to 200 RPS
        { duration: "5s", target: 600 }, // Ramp up to 600 RPS
        { duration: "5s", target: 800 }, // Ramp up to 800 RPS
        { duration: "5s", target: 1000 }, // Ramp up to 1000 RPS
        { duration: "30s", target: 2000 }, // Ramp up to 2000 RPS
        { duration: "30s", target: 3000 }, // Ramp up to 3000 RPS
        { duration: "30s", target: 5000 }, // Ramp up to 5000 RPS
        { duration: "30s", target: 10000 }, // Ramp up to 10000 RPS
        { duration: "30s", target: 20000 }, // Ramp up to 20000 RPS
        { duration: "5s", target: 1000 }, // Ramp down to 1000 RPS
      ],
    },

    // Spike test - sudden burst of traffic
    // spike_test: {
    //   executor: "ramping-arrival-rate",
    //   startRate: 0,
    //   timeUnit: "1s",
    //   preAllocatedVUs: 50,
    //   maxVUs: 200,
    //   stages: [
    //     { duration: "5s", target: 0 }, // No load for 5 seconds
    //     { duration: "10s", target: 1000 }, // Sudden spike to 1000 RPS
    //     { duration: "1m", target: 1000 }, // Maintain spike for 1 minute
    //     { duration: "5s", target: 0 }, // Sudden drop to 0 RPS
    //   ],
    // },
  },

  thresholds: {
    http_req_duration: ["p(95)<500"], // 95% of requests should be below 500ms
    http_req_failed: ["rate<0.1"], // Error rate should be less than 10%
    errors: ["rate<0.1"], // Custom error rate should be less than 10%

    // Error counter thresholds
    client_errors: ["count<100"], // Less than 100 client errors total
    server_errors: ["count<50"], // Less than 50 server errors total
    validation_errors: ["count<20"], // Less than 20 validation errors total
    rate_limit_errors: ["count<5"], // Less than 5 rate limit errors total
    internal_server_errors: ["count<10"], // Less than 10 internal server errors total
  },
};

// Test data generation
function generateUserData() {
  const id = crypto.randomUUID();
  return {
    name: `User ${id}`,
    email: `user${id}@example.com`,
  };
}

export default function () {
  const baseUrl = __ENV.BASE_URL || "http://localhost:8080";
  const userData = generateUserData();

  const payload = JSON.stringify(userData);
  const params = {
    timeout: "3s",
    headers: {
      "Content-Type": "application/json",
    },
  };

  http.setResponseCallback(only201Status);

  // Create user request
  const response = http.post(`${baseUrl}/users`, payload, params);

  // Increment error counters based on status code
  if (response.status >= 400) {
    // Increment general error category counters
    if (response.status >= 500) {
      errorCounters.serverErrors.add(1);
    } else {
      errorCounters.clientErrors.add(1);
    }

    // Increment specific status code counter if mapped
    const specificCounter = statusCodeToCounter.get(response.status);
    if (specificCounter) {
      specificCounter.add(1);
    }
  }

  // Handle network/timeout errors
  if (response.timings.duration > 3000) {
    errorCounters.timeoutErrors.add(1);
  }

  // Check response
  const success = check(response, {
    "status is 201": (r) => r.status === 201,
    "response has user id": (r) => {
      try {
        const body = r.json() as { id: string };
        return body.id !== undefined;
      } catch {
        return false;
      }
    },
    "response time < 500ms": (r) => r.timings.duration < 500,
  });

  errorRate.add(!success);
}

// Setup function (runs once before the test)
export function setup() {
  console.log("Starting user creation load test");
  console.log(`Base URL: ${__ENV.BASE_URL || "http://localhost:8080"}`);
}

export function teardown(data: any) {
  console.log("User creation load test completed");
}
