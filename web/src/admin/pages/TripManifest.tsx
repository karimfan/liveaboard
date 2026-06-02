import { useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";

import {
  adminApi,
  type TripCabinBoard,
  type TripLifecycle,
  type TripManifest as TripManifestData,
} from "../api";
import { useDevFlags } from "../../lib/devFlags";
import { fakeGuestIdentity } from "../../lib/fakeData";
import {
  Button,
  Card,
  Chip,
  type ChipVariant,
  type Column,
  DataTable,
  Field,
  PageHeader,
  Stat,
} from "../components";

import styles from "./TripManifest.module.css";

export function TripManifest() {
  const { id = "" } = useParams<{ id: string }>();
  const devFlags = useDevFlags();
  const [data, setData] = useState<TripManifestData | null>(null);
  const [board, setBoard] = useState<TripCabinBoard | null>(null);
  const [lifecycle, setLifecycle] = useState<TripLifecycle | null>(null);
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [berthId, setBerthId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [filling, setFilling] = useState(false);
  const [transitioning, setTransitioning] = useState(false);
  const [transitionReason, setTransitionReason] = useState("");

  async function load() {
    if (!id) return;
    setError(null);
    // Manifest is essential; the page can't render without it. Cabin
    // board and lifecycle are best-effort — a failure on either leaves
    // the manifest visible with a callout instead of blanking the
    // whole page.
    try {
      const manifest = await adminApi.tripManifest(id);
      setData(manifest);
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Failed to load manifest.",
      );
      return;
    }
    adminApi
      .tripCabinBoard(id)
      .then(setBoard)
      .catch(() => setBoard(null));
    adminApi
      .tripLifecycle(id)
      .then(setLifecycle)
      .catch(() => setLifecycle(null));
  }

  useEffect(() => {
    void load();
  }, [id]);

  async function addGuest(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setMessage(null);
    try {
      await adminApi.addTripGuest(id, {
        full_name: fullName,
        email,
        berth_id: berthId,
      });
      setFullName("");
      setEmail("");
      setBerthId("");
      setMessage("Registration invite sent.");
      await load();
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Failed to add guest.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  // fillBoat is the dev-only "fill the boat with test guests"
  // action. It walks the available berths and creates one fake guest
  // per berth, sequentially so we don't race a unique-email retry.
  // If a single create fails, we stop and surface the message, but
  // keep the guests we already created.
  async function fillBoat() {
    if (filling) return;
    const berths = availableBerths(board);
    if (berths.length === 0) {
      setError("No available berths to fill.");
      return;
    }
    setFilling(true);
    setError(null);
    setMessage(null);
    let created = 0;
    try {
      for (const b of berths) {
        const ident = fakeGuestIdentity();
        await adminApi.addTripGuest(id, {
          full_name: ident.full_name,
          email: ident.email,
          berth_id: b.id,
        });
        created++;
      }
      setMessage(
        `Filled boat with ${created} test ${created === 1 ? "guest" : "guests"}.`,
      );
    } catch (err) {
      setError(
        (err as { message?: string })?.message ??
          `Failed after creating ${created} ${created === 1 ? "guest" : "guests"}.`,
      );
    } finally {
      setFilling(false);
      await load();
    }
  }

  async function resend(guestId: string) {
    setError(null);
    setMessage(null);
    try {
      await adminApi.resendTripGuestInvite(id, guestId);
      setMessage("Registration invite resent.");
      await load();
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Failed to resend invite.",
      );
    }
  }

  async function revoke(guestId: string) {
    setError(null);
    setMessage(null);
    try {
      await adminApi.revokeTripGuestInvite(id, guestId);
      setMessage("Invite revoked.");
      await load();
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Failed to revoke invite.",
      );
    }
  }

  async function transition(action: "start" | "complete" | "cancel") {
    setTransitioning(true);
    setError(null);
    setMessage(null);
    try {
      const acknowledged = [
        ...new Set((lifecycle?.readiness.warnings ?? []).map((w) => w.code)),
      ];
      if (action === "start") {
        await adminApi.startTrip(id, {
          acknowledged_warnings: acknowledged,
          reason: transitionReason,
        });
        setMessage("Trip started.");
      } else if (action === "complete") {
        await adminApi.completeTrip(id, {
          acknowledged_warnings: acknowledged,
          reason: transitionReason,
        });
        setMessage("Trip completed.");
      } else {
        await adminApi.cancelTrip(id, { reason: transitionReason });
        setMessage("Trip cancelled.");
      }
      setTransitionReason("");
      await load();
    } catch (err) {
      setError(
        (err as { message?: string })?.message ??
          "Could not update trip lifecycle.",
      );
    } finally {
      setTransitioning(false);
    }
  }

  if (!data) {
    return (
      <>
        <div className={styles.breadcrumb}>
          <Link to="/admin/trips">Trips</Link>
        </div>
        {error ? (
          <div className={styles.error}>{error}</div>
        ) : (
          <div className={styles.muted}>Loading...</div>
        )}
      </>
    );
  }

  const formDisabled =
    submitting ||
    lifecycle?.trip.status === "completed" ||
    lifecycle?.trip.status === "cancelled";

  const guestColumns: Column<
    NonNullable<TripManifestData["guests"]>[number]
  >[] = [
    {
      key: "guest",
      header: "Guest",
      cell: (g) => (
        <Link to={`/admin/trips/${id}/guests/${g.id}`}>{g.full_name}</Link>
      ),
    },
    { key: "email", header: "Email", cell: (g) => g.email },
    {
      key: "cabin",
      header: "Cabin",
      cell: (g) =>
        g.cabin_assignment?.display_label ?? (
          <Chip variant="warning">Needs cabin</Chip>
        ),
    },
    {
      key: "status",
      header: "Status",
      cell: (g) => (
        <Chip variant={guestStatusVariant(g.status)}>
          {statusLabel(g.status)}
        </Chip>
      ),
    },
    {
      key: "invite",
      header: "Invite",
      cell: (g) =>
        g.invite_last_error ? (
          <span className={styles.errorInline}>{g.invite_last_error}</span>
        ) : (
          (g.invite_expires_at ?? "—")
        ),
    },
    {
      key: "actions",
      header: "",
      cell: (g) => (
        <div className={styles.actionsCell}>
          <Link to={`/admin/trips/${id}/guests/${g.id}`}>Details</Link>
          <Button variant="secondary" onClick={() => resend(g.id)}>
            Resend
          </Button>
          <Button variant="quiet" onClick={() => revoke(g.id)}>
            Revoke
          </Button>
          <Link to={`/admin/trips/${id}/guests/${g.id}/folio`}>Checkout</Link>
        </div>
      ),
    },
  ];

  return (
    <>
      <div className={styles.breadcrumb}>
        <Link to="/admin/trips">Trips</Link>
      </div>
      <PageHeader
        title="Manifest"
        subtitle={`${data.trip.boat_name} - ${data.trip.start_date} to ${data.trip.end_date} - ${data.trip.itinerary}`}
        actions={
          <>
            {lifecycle?.trip.status === "active" && (
              <Link to={`/admin/trips/${id}/ledger`}>
                <Button variant="primary">Ledger</Button>
              </Link>
            )}
            <Link to={`/admin/trips/${id}/dashboard`}>
              <Button variant="secondary">Dashboard</Button>
            </Link>
            <Link to={`/admin/trips/${id}/cabins`}>
              <Button variant="secondary">Cabin board</Button>
            </Link>
          </>
        }
      />

      {error && <div className={styles.error}>{error}</div>}
      {message && <div className={styles.callout}>{message}</div>}

      {lifecycle && (
        <Card
          title="Lifecycle"
          actions={
            <Chip variant={tripStatusVariant(lifecycle.trip.status)}>
              {lifecycle.trip.status}
            </Chip>
          }
        >
          <div className={styles.lifecycleIssues}>
            {(lifecycle.readiness.blockers ?? []).map((issue, i) => (
              <div key={`b-${i}`} className={styles.errorInline}>
                {issue.message}
              </div>
            ))}
            {(lifecycle.readiness.warnings ?? []).map((issue, i) => (
              <div key={`w-${i}`} className={styles.muted}>
                Warning: {issue.message}
              </div>
            ))}
            {(lifecycle.readiness.blockers ?? []).length === 0 &&
              (lifecycle.readiness.warnings ?? []).length === 0 && (
                <div className={styles.muted}>
                  No lifecycle blockers or warnings.
                </div>
              )}
          </div>
          <div className={styles.lifecycleActions}>
            <input
              className={styles.input}
              value={transitionReason}
              onChange={(e) => setTransitionReason(e.target.value)}
              placeholder="Reason required for warnings, override, or cancellation"
            />
            {lifecycle.trip.status === "planned" && (
              <>
                <Button
                  variant="secondary"
                  disabled={
                    transitioning ||
                    (lifecycle.readiness.blockers ?? []).length > 0
                  }
                  onClick={() => transition("start")}
                >
                  Start trip
                </Button>
                <Button
                  variant="quiet"
                  disabled={transitioning}
                  onClick={() => transition("cancel")}
                >
                  Cancel
                </Button>
              </>
            )}
            {lifecycle.trip.status === "active" && (
              <Button
                variant="secondary"
                disabled={transitioning}
                onClick={() => transition("complete")}
              >
                Complete trip
              </Button>
            )}
          </div>
        </Card>
      )}

      <div className={styles.summary}>
        <Stat label="Guests" value={data.summary.guest_count} />
        <Stat label="Submitted" value={data.summary.submitted_count} />
        <Stat label="Expected" value={data.summary.expected_count ?? "—"} />
        {data.summary.has_warning && (
          <div className={styles.summaryWarning}>Above expected count</div>
        )}
      </div>

      <Card title="Add guest">
        <form onSubmit={addGuest}>
          {devFlags.filesystem_email &&
            board !== null &&
            availableBerths(board).length > 0 && (
              <div className={styles.calloutInline}>
                <strong>Dev affordance:</strong>{" "}
                <Button
                  variant="secondary"
                  disabled={filling}
                  onClick={() => void fillBoat()}
                >
                  {filling
                    ? "Filling…"
                    : `Fill boat with ${availableBerths(board).length} test guests`}
                </Button>
                <span className={styles.calloutNote}>
                  One synthetic guest per available berth — emails go to
                  /tmp/inbox.
                </span>
              </div>
            )}
          {board !== null && availableBerths(board).length === 0 ? (
            <div className={styles.callout}>
              No cabin berths available yet. Set up the boat's cabin layout
              first:{" "}
              <Link to={`/admin/fleet/${data.trip.boat_id}/cabins`}>
                Cabin layout for {data.trip.boat_name}
              </Link>
              .
            </div>
          ) : (
            <div className={styles.formGrid}>
              <Field label="Full name" htmlFor="guest-name">
                <input
                  className={styles.input}
                  id="guest-name"
                  value={fullName}
                  onChange={(e) => setFullName(e.target.value)}
                  required
                />
              </Field>
              <Field label="Email" htmlFor="guest-email">
                <input
                  className={styles.input}
                  id="guest-email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </Field>
              <Field label="Cabin berth" htmlFor="guest-berth">
                <select
                  className={styles.input}
                  id="guest-berth"
                  value={berthId}
                  onChange={(e) => setBerthId(e.target.value)}
                  required
                >
                  <option value="">Select berth...</option>
                  {availableBerths(board).map((b) => (
                    <option key={b.id} value={b.id}>
                      {b.label}
                    </option>
                  ))}
                </select>
              </Field>
              <Button variant="primary" type="submit" disabled={formDisabled}>
                {submitting ? "Sending..." : "Send invite"}
              </Button>
            </div>
          )}
        </form>
      </Card>

      <DataTable
        columns={guestColumns}
        rows={data.guests ?? []}
        rowKey={(g) => g.id}
      />
    </>
  );
}

function statusLabel(s: string): string {
  return s.replaceAll("_", " ");
}

function tripStatusVariant(status: string): ChipVariant {
  switch (status) {
    case "active":
      return "success";
    case "completed":
      return "info";
    case "cancelled":
      return "error";
    default:
      return "neutral";
  }
}

function guestStatusVariant(status: string): ChipVariant {
  switch (status) {
    case "submitted":
      return "info";
    case "pending":
      return "warning";
    case "cancelled":
    case "removed":
      return "error";
    case "active":
      return "success";
    default:
      return "neutral";
  }
}

function availableBerths(
  board: TripCabinBoard | null,
): { id: string; label: string }[] {
  if (!board || !board.cabins) return [];
  return board.cabins.flatMap((c) =>
    (c.berths ?? [])
      .filter((b) => !b.guest)
      .map((b) => ({ id: b.id, label: `${c.label} - ${b.display_label}` })),
  );
}
