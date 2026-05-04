import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { ClerkProvider } from "@clerk/clerk-react";

import "./styles/tokens.css";
import "./styles/app.css";

import { Login } from "./pages/Login";
import { Signup } from "./pages/Signup";
import { Dashboard } from "./pages/Dashboard";
import { RequireSession } from "./lib/RequireSession";
import { appConfig } from "./lib/config";
import { clerkAppearance } from "./lib/clerkAppearance";

import { AdminShell, RequireAdmin } from "./admin/Shell";
import { MeProvider } from "./admin/useMe";
import { Overview } from "./admin/pages/Overview";
import { Fleet } from "./admin/pages/Fleet";
import { BoatDetail } from "./admin/pages/BoatDetail";
import { BoatTrips, BoatInventory, BoatNotes } from "./admin/pages/BoatTabs";
import { Trips } from "./admin/pages/Trips";
import { Catalog } from "./admin/pages/Catalog";
import { Users } from "./admin/pages/Users";
import { Organization } from "./admin/pages/Organization";
import { Reports } from "./admin/pages/Reports";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ClerkProvider
      publishableKey={appConfig.clerkPublishableKey}
      appearance={clerkAppearance}
      signInFallbackRedirectUrl="/"
      signUpFallbackRedirectUrl="/signup"
    >
      <BrowserRouter>
        <Routes>
          <Route path="/signup/*" element={<Signup />} />
          <Route path="/login/*" element={<Login />} />
          <Route
            path="/"
            element={
              <RequireSession>
                <Dashboard />
              </RequireSession>
            }
          />
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

            {/* Org-admin-only routes — RequireAdmin redirects directors to /admin */}
            <Route
              path="organization"
              element={<RequireAdmin><Organization /></RequireAdmin>}
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
              path="catalog"
              element={<RequireAdmin><Catalog /></RequireAdmin>}
            />
            <Route
              path="users"
              element={<RequireAdmin><Users /></RequireAdmin>}
            />
            <Route
              path="reports"
              element={<RequireAdmin><Reports /></RequireAdmin>}
            />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </ClerkProvider>
  </React.StrictMode>,
);
