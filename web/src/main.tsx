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

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ClerkProvider
      publishableKey={appConfig.clerkPublishableKey}
      // Tell Clerk where to send the browser after the hosted flows
      // complete; both routes are within our SPA and handle the next
      // step (org bootstrap or session exchange) themselves.
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
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </ClerkProvider>
  </React.StrictMode>,
);
