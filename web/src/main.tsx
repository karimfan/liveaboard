import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import "./styles/tokens.css";
import "./styles/app.css";

import { Login } from "./pages/Login";
import { Signup } from "./pages/Signup";
import { VerifyEmail } from "./pages/VerifyEmail";
import { ForgotPassword } from "./pages/ForgotPassword";
import { ResetPassword } from "./pages/ResetPassword";
import { AcceptInvitation } from "./pages/AcceptInvitation";
import { GuestInvitation } from "./pages/GuestInvitation";
import { GuestRegistration } from "./pages/GuestRegistration";
import { RequireSession } from "./lib/RequireSession";

import { AdminShell, RequireAdmin } from "./admin/Shell";
import { MeProvider } from "./admin/useMe";
import { Overview } from "./admin/pages/Overview";
import { Fleet } from "./admin/pages/Fleet";
import { BoatDetail } from "./admin/pages/BoatDetail";
import { BoatTrips, BoatInventory, BoatNotes } from "./admin/pages/BoatTabs";
import { Trips } from "./admin/pages/Trips";
import { TripManifest } from "./admin/pages/TripManifest";
import { Inventory } from "./admin/pages/Inventory";
import { Users } from "./admin/pages/Users";
import { Organization } from "./admin/pages/Organization";
import { OrganizationPayments } from "./admin/pages/OrganizationPayments";
import { GuestFolio } from "./admin/pages/GuestFolio";
import { Reports } from "./admin/pages/Reports";
import { Account } from "./admin/pages/Account";
import { Import } from "./admin/pages/Import";
import { ImportLiveaboard } from "./admin/pages/ImportLiveaboard";
import { ImportSpreadsheet } from "./admin/pages/ImportSpreadsheet";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        {/* Public auth surface — no session required. */}
        <Route path="/signup" element={<Signup />} />
        <Route path="/login" element={<Login />} />
        <Route path="/verify-email" element={<VerifyEmail />} />
        <Route path="/forgot-password" element={<ForgotPassword />} />
        <Route path="/reset-password" element={<ResetPassword />} />
        <Route path="/invitations/:token/accept" element={<AcceptInvitation />} />
        <Route path="/guest/invitations/:token" element={<GuestInvitation />} />
        <Route path="/guest/trips/:tripGuestId/register" element={<GuestRegistration />} />

        {/* Root redirects to the admin chrome — that's the only authenticated
            surface. RequireSession on /admin handles the unauthenticated case. */}
        <Route path="/" element={<Navigate to="/admin" replace />} />
        <Route
          path="/admin"
          element={
            <RequireSession>
              <MeProvider>
                <AdminShell />
              </MeProvider>
            </RequireSession>
          }
        >
          {/* Both roles */}
          <Route index element={<Overview />} />
          <Route path="trips" element={<Trips />} />
          <Route path="trips/:id/manifest" element={<TripManifest />} />
          <Route path="trips/:id/guests/:guestId/folio" element={<GuestFolio />} />
          <Route path="account" element={<Account />} />

          {/* Org-admin-only routes — RequireAdmin redirects directors to /admin */}
          <Route
            path="organization"
            element={<RequireAdmin><Organization /></RequireAdmin>}
          />
          <Route
            path="organization/payments"
            element={<RequireAdmin><OrganizationPayments /></RequireAdmin>}
          />
          <Route
            path="fleet"
            element={<RequireAdmin><Fleet /></RequireAdmin>}
          />
          <Route
            path="fleet/:id"
            element={<RequireAdmin><BoatDetail /></RequireAdmin>}
          >
            <Route index element={<BoatTrips />} />
            <Route path="inventory" element={<BoatInventory />} />
            <Route path="notes" element={<BoatNotes />} />
          </Route>
          <Route
            path="inventory"
            element={<RequireAdmin><Inventory /></RequireAdmin>}
          />
          <Route
            path="users"
            element={<RequireAdmin><Users /></RequireAdmin>}
          />
          <Route
            path="reports"
            element={<RequireAdmin><Reports /></RequireAdmin>}
          />
          <Route
            path="import"
            element={<RequireAdmin><Import /></RequireAdmin>}
          />
          <Route
            path="import/liveaboard"
            element={<RequireAdmin><ImportLiveaboard /></RequireAdmin>}
          />
          <Route
            path="import/spreadsheet"
            element={<RequireAdmin><ImportSpreadsheet /></RequireAdmin>}
          />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
);
