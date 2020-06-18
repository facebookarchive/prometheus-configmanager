// @prettier
// @flow

import React from 'react';
import amber from '@material-ui/core/colors/amber';
import green from '@material-ui/core/colors/green';
import MuiStylesThemeProvider from '@material-ui/styles/ThemeProvider';
import TenantSelector from './TenantSelector';

import {BrowserRouter, Redirect, Route} from 'react-router-dom';
import {APIUtil} from './APIUtil';
import {Alarms} from '@fbcnms/alarms';
import {createMuiTheme} from '@material-ui/core/styles';
import {MuiThemeProvider} from '@material-ui/core/styles';
import {SnackbarProvider} from 'notistack';
import {useState} from 'react';

import type {Labels, TenancyConfig} from '@fbcnms/alarms/components/AlarmAPIType';

// default theme
const theme = createMuiTheme({
  palette: {
    success: {
      light: green[100],
      main: green[500],
      dark: green[800],
    },
    warning: {
      light: amber[100],
      main: amber[500],
      dark: amber[800],
    },
    // symphony theming
    secondary: {
      main: '#303846',
    },
    grey: {
      '50': '#e4f0f6',
    },
  },
});


function AlarmsMain() {
  const [tenantID, setTenantID] = useState<string>("default");

  const apiUtil = React.useMemo(() => APIUtil(tenantID),[tenantID])

  // Initialize tenant if not already initialized
  const {error} = apiUtil.useAlarmsApi(apiUtil.getRouteTree, {tenantID})
  React.useEffect(() => {
    if (error?.response?.status === 400 &&
        error?.response?.data?.message?.includes('does not exist')) {
      APIUtil(tenantID).editRouteTree({
        route: {
          receiver: `${tenantID}_tenant_base_route`,
        }
      })
    }
  }, [error, tenantID])


  const {response: amTenancyResp} = apiUtil.useAlarmsApi(apiUtil.getAlertmanagerTenancy, {})
  const {response: promTenancyResp} = apiUtil.useAlarmsApi(apiUtil.getPrometheusTenancy, {})

  const amTenancy: TenancyConfig = amTenancyResp ?? {restrictor_label: "", restrict_queries: false};
  const promTenancy: TenancyConfig = promTenancyResp ?? {restrictor_label: "tenant", restrict_queries: false};

  const isSingleTenant = amTenancy.restrictor_label === "";

  const filterLabels = (labels: Labels): Labels => {
    const labelsToFilter = ['monitor', 'instance', 'job'];
    isSingleTenant && labelsToFilter.push(promTenancy.restrictor_label);
    const filtered = {...labels};
    for (const label of labelsToFilter) {
      delete filtered[label];
    }
    return filtered;
  }

  return(
    <>
      <MuiThemeProvider theme={theme}>
        <MuiStylesThemeProvider theme={theme}>
          <SnackbarProvider
            maxSnack={3}
            preventDuplicate
            anchorOrigin={{
              vertical: 'bottom',
              horizontal: 'right',
          }}>
          {isSingleTenant ||
          <TenantSelector apiUtil={apiUtil} setTenantId={setTenantID} tenantID={tenantID}/>
          }
          <Alarms
            apiUtil={apiUtil}
            makeTabLink={({match, keyName}) => {
                return `${keyName}`
              }
            }
            alertManagerGlobalConfigEnabled={true}
            disabledTabs={['suppressions', 'routes']}
            thresholdEditorEnabled={true}
            filterLabels={filterLabels}
          />
          </SnackbarProvider>
        </MuiStylesThemeProvider>
      </MuiThemeProvider>
    </>
  )
}


function App() {
  return (
    <div>
      <BrowserRouter >
        <Route path={"/alarms"} exact={false} render={() => <AlarmsMain/>}>
        </Route>
        <Redirect to={"/alarms"}></Redirect>
      </BrowserRouter>
    </div>
  );
}

export default App;
