/**
 * Copyright 2004-present Facebook. All Rights Reserved.
 *
 * @format
 * @flow strict-local
 */

import axios from 'axios';
import {useEffect, useState} from 'react';
import type {ApiUtil} from '@fbcnms/alarms/components/AlarmsApi';
import type {AxiosXHRConfig} from 'axios';

export const AM_BASE_URL = process.env.REACT_APP_AM_BASE_URL || '';
export const PROM_BASE_URL = process.env.REACT_APP_PROM_BASE_URL || '';
export const AM_CONFIG_URL = process.env.REACT_APP_AM_CONFIG_URL || '';
export const PROM_CONFIG_URL = process.env.REACT_APP_PROM_CONFIG_URL || '';

export function APIUtil(tenantID: string): ApiUtil {
  return {
    useAlarmsApi: useApi,

    // Alertmanager Requests
    viewFiringAlerts: _req =>
      makeRequest({
        url: `${AM_BASE_URL}/alerts`,
      }),
    viewMatchingAlerts: ({expression}) =>
      makeRequest({url: `${AM_BASE_URL}/matching_alerts/${expression}`}),

    // suppressions
    getSuppressions: _req =>
      makeRequest({
        url: `${AM_BASE_URL}/silences`,
        method: 'GET',
      }),
    // global config
    getGlobalConfig: _req =>
      makeRequest({url: `${AM_BASE_URL}/globalconfig`, method: 'GET'}),


    // Prometheus Configmanager Requests
    createAlertRule: ({rule}) =>
      makeRequest({
        url: `${PROM_CONFIG_URL}/${tenantID}/alert`,
        method: 'POST',
        data: rule,
      }),
    editAlertRule: ({rule}) =>
      makeRequest({
        url: `${PROM_CONFIG_URL}/${tenantID}/alert/${rule.alert}`,
        data: rule,
        method: 'PUT',
      }),
    getAlertRules: _req =>
      makeRequest({
        url: `${PROM_CONFIG_URL}/${tenantID}/alert`,
        method: 'GET',
      }),
    deleteAlertRule: ({ruleName}) =>
      makeRequest({
        url: `${PROM_CONFIG_URL}/${tenantID}/alert/${ruleName}`,
        method: 'DELETE',
      }),

    // Alertmanager Configurer Requests
    // receivers
    createReceiver: ({receiver}) =>
      makeRequest({
        url: `${AM_CONFIG_URL}/${tenantID}/receiver`,
        method: 'POST',
        data: receiver,
      }),
    editReceiver: ({receiver}) =>
      makeRequest({
        url: `${AM_CONFIG_URL}/${tenantID}/receiver/${receiver.name}`,
        method: 'PUT',
        data: receiver,
      }),
    getReceivers: _req =>
      makeRequest({
        url: `${AM_CONFIG_URL}/${tenantID}/receiver`,
        method: 'GET',
      }),
    deleteReceiver: ({receiverName}) =>
      makeRequest({
        url: `${AM_CONFIG_URL}/${tenantID}/receiver/${receiverName}`,
        method: 'DELETE',
      }),

    // routes
    getRouteTree: _req =>
      makeRequest({
        url: `${AM_CONFIG_URL}${tenantID}/route`,
        method: 'GET',
      }),
    editRouteTree: req =>
      makeRequest({
        url: `${AM_CONFIG_URL}/${tenantID}/route`,
        method: 'POST',
        data: req.route,
      }),

    // metric series
    getMetricSeries: _req =>
      makeRequest({
        url: `${PROM_BASE_URL}/series?match[]={__name__=~".*"}`,
        method: 'GET',
      }),

    editGlobalConfig: ({config}) =>
      makeRequest({
        url: `${AM_CONFIG_URL}/${tenantID}/globalconfig`,
        method: 'POST',
        data: config,
      }),
    getTenants: _req =>
      makeRequest({
        url: `${AM_CONFIG_URL}/tenants`,
        method: 'GET',
      })
    }
};

function useApi<TParams: {...}, TResponse>(
  func: TParams => Promise<TResponse>,
  params: TParams,
  cacheCounter?: string | number,
): {
  response: ?TResponse,
  error: ?Error,
  isLoading: boolean,
} {
  const [response, setResponse] = useState();
  const [error, setError] = useState<?Error>(null);
  const [isLoading, setIsLoading] = useState(true);
  const jsonParams = JSON.stringify(params);

  useEffect(() => {
    async function makeRequest() {
      try {
        const parsed = JSON.parse(jsonParams);
        setIsLoading(true);
        const res = await func(parsed);
        setResponse(res);
        setError(null);
        setIsLoading(false);
      } catch (err) {
        setError(err);
        setResponse(null);
        setIsLoading(false);
      }
    }
    makeRequest();
  }, [jsonParams, func, cacheCounter]);

  return {
    error,
    response,
    isLoading,
  };
}

async function makeRequest<TParams, TResponse>(
  axiosConfig: AxiosXHRConfig<TParams, TResponse>,
): Promise<TResponse> {
  const response = await axios(axiosConfig);
  return response.data;
}
