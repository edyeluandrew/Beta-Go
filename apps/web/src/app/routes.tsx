import { Navigate, Route, Routes } from 'react-router-dom'
import { AdminHome } from '../features/admin/AdminHome'
import { LaunchPage } from '../features/admin/LaunchPage'
import { MoneyPage } from '../features/admin/MoneyPage'
import { VendorDetailPage } from '../features/admin/VendorDetailPage'
import { VendorsPage } from '../features/admin/VendorsPage'
import { JoinPage } from '../features/auth/JoinPage'
import { SignInPage } from '../features/auth/SignInPage'
import { CustomerAccountPage } from '../features/customer/CustomerAccountPage'
import { CustomerHome } from '../features/customer/CustomerHome'
import { LandingPage } from '../features/customer/LandingPage'
import { OrderPage } from '../features/customer/OrderPage'
import { OrdersPage } from '../features/customer/OrdersPage'
import { ShopPage } from '../features/customer/ShopPage'
import { AcceptInvitePage } from '../features/vendor/AcceptInvitePage'
import { VendorHome } from '../features/vendor/VendorHome'
import { VendorOrderPage } from '../features/vendor/VendorOrderPage'
import { VendorOrdersPage } from '../features/vendor/VendorOrdersPage'
import { VendorPoolPage } from '../features/vendor/VendorPoolPage'
import { staffRoles } from '../shared/auth/roles'
import { RequireAuth, RequireRole } from './guards'

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<LandingPage />} />
      <Route path="/signin" element={<SignInPage />} />
      <Route path="/join" element={<JoinPage />} />
      <Route path="/c" element={<CustomerHome />} />
      <Route path="/c/shops/:id" element={<ShopPage />} />
      <Route element={<RequireAuth />}>
        <Route
          path="/c/account"
          element={
            <RequireRole roles={['customer']}>
              <CustomerAccountPage />
            </RequireRole>
          }
        />
        <Route
          path="/c/orders"
          element={
            <RequireRole roles={['customer']}>
              <OrdersPage />
            </RequireRole>
          }
        />
        <Route
          path="/c/orders/:id"
          element={
            <RequireRole roles={['customer']}>
              <OrderPage />
            </RequireRole>
          }
        />
        <Route path="/invite" element={<AcceptInvitePage />} />
        <Route
          path="/v"
          element={
            <RequireRole roles={['vendor']}>
              <VendorHome />
            </RequireRole>
          }
        />
        <Route
          path="/v/orders"
          element={
            <RequireRole roles={['vendor']}>
              <VendorOrdersPage />
            </RequireRole>
          }
        />
        <Route
          path="/v/orders/:id"
          element={
            <RequireRole roles={['vendor']}>
              <VendorOrderPage />
            </RequireRole>
          }
        />
        <Route
          path="/v/pool"
          element={
            <RequireRole roles={['vendor']}>
              <VendorPoolPage />
            </RequireRole>
          }
        />
        <Route
          path="/a"
          element={
            <RequireRole roles={staffRoles}>
              <AdminHome />
            </RequireRole>
          }
        />
        <Route
          path="/a/vendors"
          element={
            <RequireRole roles={['support', 'super_admin']}>
              <VendorsPage />
            </RequireRole>
          }
        />
        <Route
          path="/a/vendors/:id"
          element={
            <RequireRole roles={['support', 'super_admin']}>
              <VendorDetailPage />
            </RequireRole>
          }
        />
        <Route
          path="/a/money"
          element={
            <RequireRole roles={['finance', 'super_admin']}>
              <MoneyPage />
            </RequireRole>
          }
        />
        <Route
          path="/a/money/:id"
          element={
            <RequireRole roles={['finance', 'super_admin']}>
              <MoneyPage />
            </RequireRole>
          }
        />
        <Route
          path="/a/launch"
          element={
            <RequireRole roles={['super_admin']}>
              <LaunchPage />
            </RequireRole>
          }
        />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
