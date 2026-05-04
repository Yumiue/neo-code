/**
 * electron-builder 配置
 */
const config = {
	appId: 'com.neocode.app',
	productName: 'NeoCode',
	directories: {
		output: 'release',
		buildResources: 'build',
	},
	files: [
		'dist/**/*',
		'dist-electron/**/*',
	],
	extraResources: [
		{
			from: 'build',
			to: '.',
			filter: ['neocode-gateway', 'neocode-gateway.exe'],
		},
	],
	win: {
		target: [
			{
				target: 'nsis',
				arch: ['x64'],
			},
		],
		artifactName: '${productName}-${version}-Setup.${ext}',
	},
	nsis: {
		oneClick: false,
		perMachine: false,
		allowToChangeInstallationDirectory: true,
		deleteAppDataOnUninstall: false,
	},
	mac: {
		target: [
			{
				target: 'dmg',
				arch: ['x64', 'arm64'],
			},
		],
		artifactName: '${productName}-${version}-${arch}.${ext}',
	},
	linux: {
		target: [
			{
				target: 'AppImage',
				arch: ['x64'],
			},
		],
		artifactName: '${productName}-${version}.${ext}',
	},
}

module.exports = config
