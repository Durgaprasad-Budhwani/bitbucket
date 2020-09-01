import React, { useEffect, useState } from 'react';
import Icon from '@pinpt/uic.next/Icon';
import Loader from '@pinpt/uic.next/Loader';
import ErrorPage from '@pinpt/uic.next/Error';
import {
	useIntegration,
	Account,
	AccountsTable,
	IntegrationType,
	OAuthConnect,
	IAuth,
	IAppBasicAuth,
	Form,
	FormType,
	Http,
	IOAuth2Auth,
	ConfigAccount,
} from '@pinpt/agent.websdk';

import styles from './styles.module.less';

interface workspacesResponse {
	is_private: Boolean;
	name: string;
	slug: string;
	type: string;
	uuid: string;
}

function createAuthHeader(auth: IAppBasicAuth | IOAuth2Auth): string {
	if ('username' in auth) {
		var basic = (auth as IAppBasicAuth);
		return 'Basic ' + btoa(basic.username + ':' + basic.password);
	}
	const oauth = (auth as IOAuth2Auth);
	return 'Bearer ' + oauth.access_token;
}

async function fetchWorkspaces(auth: IAppBasicAuth | IOAuth2Auth): Promise<workspacesResponse[]> {
	try {
		const url = auth.url + '/2.0/workspaces';
		const res = await Http.get(url, { 'Authorization': createAuthHeader(auth) });
		if (res?.[1] === 200) {
			return res[0].values;
		}
		throw new Error("error fetching workspaces, response code: " + res[0]);
	} catch (err) {
		throw new Error("error fetching workspaces, check credentials");
	}
}

interface validateResponse {
	accounts: ConfigAccount[];
}

const toAccount = (data: ConfigAccount): Account => {
	return {
		id: data.id,
		public: data.public,
		type: data.type,
		avatarUrl: data.avatarUrl,
		name: data.name || '',
		description: data.description || '',
		totalCount: data.totalCount || 0,
	}
};

const AccountList = ({ workspaces, setWorkspaces }: { workspaces: workspacesResponse[], setWorkspaces: (val: workspacesResponse[]) => void }) => {
	const { config, setConfig, installed, setInstallEnabled, setValidate } = useIntegration();
	const [accounts, setAccounts] = useState<Account[]>([]);
	const [fetching, setFetching] = useState(false);
	const [error, setError] = useState<Error>();

	let auth: IAppBasicAuth | IOAuth2Auth;
	if (config.basic_auth) {
		auth = config.basic_auth as IAppBasicAuth;
	} else {
		auth = config.oauth2_auth as IOAuth2Auth;
	}

	useEffect(() => {
		if (fetching || accounts.length ) {
			return
		}
		setFetching(true);
		const fetch = async () => {
			try {
				config.accounts = {}
				const res: validateResponse = await setValidate(config);
				for (let i = 0; i < res.accounts.length; i++) {
					const obj = toAccount(res.accounts[i]);
					accounts.push(obj);
					config.accounts[obj.id] = obj;
				}
				setConfig(config);
				setAccounts(accounts)
				if (!installed && accounts.length > 0) {
					setInstallEnabled(true);
				}
			} catch (err) {
				setError(err);
			} finally {
				setFetching(false);
			}
		}
		fetch();
	}, [workspaces]);

	if (fetching) {
		return <Loader centered style={{height: '30rem'}} />;
	}
	if (error) {
		return <ErrorPage message={error.message} error={error} />;
	}
	return (
		<AccountsTable
			description='For the selected accounts, all repositories, pull requests and other data will automatically be made available in Pinpoint once installed.'
			accounts={accounts}
			entity='repo'
			config={config}
		/>
	);
};

const LocationSelector = ({ setType }: { setType: (val: IntegrationType) => void }) => {
	return (
		<div className={styles.Location}>
			<div className={styles.Button} onClick={() => setType(IntegrationType.CLOUD)}>
				<Icon icon={['fas', 'cloud']} className={styles.Icon} />
				I'm using the <strong>bitbucket.com</strong> cloud service to manage my data
			</div>

			<div className={styles.Button} onClick={() => setType(IntegrationType.SELFMANAGED)}>
				<Icon icon={['fas', 'server']} className={styles.Icon} />
				I'm using <strong>my own systems</strong> or a <strong>third-party</strong> to manage a BitBucket service
			</div>
		</div>
	);
};

const SelfManagedForm = ({ setWorkspaces }: { setWorkspaces: (val: workspacesResponse[]) => void }) => {
	async function verify(auth: IAuth) {
		try {
			const res = await fetchWorkspaces(auth as IAppBasicAuth);
			setWorkspaces(res);
		} catch (ex) {
			throw new Error(ex)
		}
	}
	return <Form type={FormType.BASIC} name='bitbucket' callback={verify} />;
};

const Integration = () => {
	const { loading, currentURL, config, isFromRedirect, isFromReAuth, setConfig } = useIntegration();
	const [type, setType] = useState<IntegrationType | undefined>(config.integration_type);
	const [, setRerender] = useState(0);
	const [workspaces, setWorkspaces] = useState<workspacesResponse[]>([]);

	// ============= OAuth 2.0 =============
	useEffect(() => {
		if (!loading && isFromRedirect && currentURL) {
			const search = currentURL.split('?');
			const tok = search[1].split('&');
			tok.forEach(async token => {
				const t = token.split('=');
				const k = t[0];
				const v = t[1];
				if (k === 'profile') {
					const profile = JSON.parse(atob(decodeURIComponent(v)));
					config.oauth2_auth = {
						date_ts: Date.now(),
						url: 'https://api.bitbucket.org',
						access_token: profile.Integration.auth.accessToken,
						refresh_token: profile.Integration.auth.refreshToken,
						scopes: profile.Integration.auth.scopes,
					};
					setConfig(config);
					setRerender(Date.now());
				}
			});
		}
	}, [config, loading, isFromRedirect, currentURL]);

	useEffect(() => {
		if (type) {
			config.integration_type = type;
			setConfig(config);
			setRerender(Date.now());
		}
	}, [config, type])

	if (loading) {
		return <Loader centered />;
	}

	let content;

	if (isFromReAuth) {
		if (config.integration_type === IntegrationType.CLOUD) {
			content = <OAuthConnect name='BitBucket' reauth />;
		} else {
			content = <SelfManagedForm setWorkspaces={setWorkspaces} />;
		}
	} else {
		if (!config.integration_type) {
			content = <LocationSelector setType={setType} />;
		} else if (config.integration_type === IntegrationType.CLOUD && !config.oauth2_auth) {
			content = <OAuthConnect name='BitBucket' />;
		} else if (config.integration_type === IntegrationType.SELFMANAGED && !config.basic_auth && !config.apikey_auth) {
			content = <SelfManagedForm setWorkspaces={setWorkspaces} />;
		} else {
			content = <AccountList workspaces={workspaces} setWorkspaces={setWorkspaces} />;
		}
	}

	return (
		<div className={styles.Wrapper}>
			{content}
		</div>
	);
};


export default Integration;