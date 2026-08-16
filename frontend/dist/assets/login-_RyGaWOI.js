import{a as e,n as t,t as n}from"./jsx-runtime-n5LQ9ujS.js";import{t as r}from"./useSearch-BuP7f8ce.js";import{t as i}from"./useNavigate-BIRE9DF9.js";import{i as a,w as o}from"./useTooltip-B0-s6ZGA.js";import{C as s,O as c,l,t as u}from"./Button-DDheN8lJ.js";import{d,t as f}from"./http-DvIQdisc.js";import{t as p}from"./Link-BhIgW1dh.js";import{n as m}from"./auth-store-Cm-_2_u_.js";import{t as h}from"./VStack-DSe210ba.js";import{t as g}from"./Card-DiUNVwjZ.js";import{t as _}from"./TextInput-Dk3gfb8Y.js";import{t as v}from"./useToast-B87utKEg.js";import{t as y}from"./auth-api-BpBQBh5a.js";import{t as b}from"./EmptyState-Co9OvEsO.js";var x=e(t());function S({title:e,titleId:t,...n},r){return x.createElement(`svg`,Object.assign({xmlns:`http://www.w3.org/2000/svg`,fill:`none`,viewBox:`0 0 24 24`,strokeWidth:1.5,stroke:`currentColor`,"aria-hidden":`true`,"data-slot":`icon`,ref:r,"aria-labelledby":t},n),e?x.createElement(`title`,{id:t},e):null,x.createElement(`path`,{strokeLinecap:`round`,strokeLinejoin:`round`,d:`M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z`}))}var C=x.forwardRef(S),w={scrollable:{kVQacm:`xysyzu8`,$$css:!0}};function T({crossAlignSelf:e,size:t,isScrollable:n,as:r=`div`,xstyle:i,className:s,style:l,children:u,ref:f,...p}){let m=o(...d({crossAlignSelf:e,size:t}),n&&w.scrollable,i);return(0,x.createElement)(r,{ref:f,...c(a(`stack-item`,{size:t}),m,s,l),...p},u)}T.displayName=`StackItem`;var E=n(),D={grid:{k1xSpc:`xrvj5dj`,$$css:!0}},O={kJBjZk:`xhij9v2`,$$css:!0},k={templateColumns:e=>[{kumcoG:e==null?e:`xqketvx`,$$css:!0},{"--x-gridTemplateColumns":e??void 0}],autoRows:e=>[O,{"--x-gridAutoRows":`${e}px`==null?void 0:`${e}px`}]},A={start:{kGNEyG:`x7a106z`,$$css:!0},center:{kGNEyG:`x6s0dn4`,$$css:!0},end:{kGNEyG:`xpqajaz`,$$css:!0},stretch:{kGNEyG:`x1qjc9v5`,$$css:!0}},j={start:{kAPf3g:`x619ttb`,$$css:!0},center:{kAPf3g:`x1o2pa38`,$$css:!0},end:{kAPf3g:`x4xo5sw`,$$css:!0},stretch:{kAPf3g:`xl4xnwh`,$$css:!0}},M={0:{kOIVth:`xsn7fz1`,$$css:!0},1:{kOIVth:`xzye2dw`,$$css:!0},2:{kOIVth:`x1txdalj`,$$css:!0},3:{kOIVth:`xjcht0a`,$$css:!0},4:{kOIVth:`x18g69wz`,$$css:!0},5:{kOIVth:`x9mgr7n`,$$css:!0},6:{kOIVth:`x1qh66ti`,$$css:!0},8:{kOIVth:`x4t41sb`,$$css:!0},10:{kOIVth:`x3hoi3v`,$$css:!0},"0.5":{kOIVth:`x1lsbc85`,$$css:!0},"1.5":{kOIVth:`x1s4dlld`,$$css:!0}},N={0:{khm7nJ:`x6yxi7o`,$$css:!0},1:{khm7nJ:`x1ngg2t4`,$$css:!0},2:{khm7nJ:`x1x7z4sm`,$$css:!0},3:{khm7nJ:`x4olc9o`,$$css:!0},4:{khm7nJ:`xtx9w7w`,$$css:!0},5:{khm7nJ:`x1iu6piu`,$$css:!0},6:{khm7nJ:`xczp1bk`,$$css:!0},8:{khm7nJ:`xgx0vcf`,$$css:!0},10:{khm7nJ:`x1xpicb7`,$$css:!0},"0.5":{khm7nJ:`x1tw44j4`,$$css:!0},"1.5":{khm7nJ:`xhq53yo`,$$css:!0}},P={0:{k1C7PZ:`x1o57wo1`,$$css:!0},1:{k1C7PZ:`x1lfs0n9`,$$css:!0},2:{k1C7PZ:`xak3so`,$$css:!0},3:{k1C7PZ:`xewh9hi`,$$css:!0},4:{k1C7PZ:`xty4p9g`,$$css:!0},5:{k1C7PZ:`x1eqhezk`,$$css:!0},6:{k1C7PZ:`x3qlgwd`,$$css:!0},8:{k1C7PZ:`xicv188`,$$css:!0},10:{k1C7PZ:`x1p37tyl`,$$css:!0},"0.5":{k1C7PZ:`x1kihgfc`,$$css:!0},"1.5":{k1C7PZ:`x1thn6ci`,$$css:!0}},F={0:`--spacing-0`,.5:`--spacing-0-5`,1:`--spacing-1`,1.5:`--spacing-1-5`,2:`--spacing-2`,3:`--spacing-3`,4:`--spacing-4`,5:`--spacing-5`,6:`--spacing-6`,8:`--spacing-8`,10:`--spacing-10`};function I(e,t,n,r,i){let a=i==null?r==null?null:F[r]:F[i];return`repeat(${n}, minmax(${`min(100%, max(${e}px, ${a?`calc((100% - ${t-1} * var(${a})) / ${t})`:`calc(100% / ${t})`}))`}, 1fr))`}function L({columns:e,minChildWidth:t=0,rowHeight:n,width:r,height:i,maxWidth:s,minHeight:l,gap:u,rowGap:d,columnGap:f,align:p,justify:m,xstyle:h,className:g,style:_,children:v,ref:y,...b}){let x;if(typeof e==`object`&&e){let t=e.repeat===`fit`?`auto-fit`:`auto-fill`;x=e.max!=null&&e.max>0?I(e.minWidth,e.max,t,u,f):`repeat(${t}, minmax(${e.minWidth}px, 1fr))`}else if(t>0){let n=typeof e==`number`?e:0;x=n>0?I(t,n,`auto-fit`,u,f):`repeat(auto-fit, minmax(${t}px, 1fr))`}else x=typeof e==`number`&&e>0?`repeat(${e}, 1fr)`:`1fr`;let S={...r!=null&&{width:typeof r==`number`?`${r}px`:r},...i!=null&&{height:typeof i==`number`?`${i}px`:i},...s!=null&&{maxWidth:typeof s==`number`?`${s}px`:s},...l!=null&&{minHeight:typeof l==`number`?`${l}px`:l}};return(0,E.jsx)(`div`,{ref:y,...c(a(`grid`,{columns:typeof e==`number`?e:void 0,gap:u,align:p,justify:m}),o(D.grid,k.templateColumns(x),n!=null&&k.autoRows(n),u!=null&&M[u],d!=null&&N[d],f!=null&&P[f],p!=null&&A[p],m!=null&&j[m],h),g,{..._,...S}),...b,children:v})}L.displayName=`Grid`;var R={base:{k1xSpc:`x78zum5`,$$css:!0},inline:{k1xSpc:`x3nfvp2`,$$css:!0},alignItemsCenter:{kGNEyG:`x6s0dn4`,$$css:!0},justifyContentCenter:{kjj79g:`xl56j7k`,$$css:!0}},z={sizing:(e,t,n,r)=>[{kzqmXN:e==null?e:`x5lhr3w`,kZKoxP:t==null?t:`x16ye13r`,ks0D6T:n==null?n:`xf68679`,kAzted:r==null?r:`x82snj4`,$$css:!0},{"--x-width":(e=>typeof e==`number`?e+`px`:e??void 0)(e),"--x-height":(e=>typeof e==`number`?e+`px`:e??void 0)(t),"--x-maxWidth":(e=>typeof e==`number`?e+`px`:e??void 0)(n),"--x-minHeight":(e=>typeof e==`number`?e+`px`:e??void 0)(r)}]};function B({axis:e=`both`,width:t,height:n,maxWidth:r,minHeight:i,isInline:s=!1,children:l,xstyle:u,className:d,style:f,ref:p,...m}){return(0,E.jsx)(`div`,{ref:p,...c(a(`center`,{axis:e}),o(s?R.inline:R.base,(e===`both`||e===`vertical`)&&R.alignItemsCenter,(e===`both`||e===`horizontal`)&&R.justifyContentCenter,z.sizing(t??null,n??null,r??null,i??null),u),d,f),...m,children:l})}B.displayName=`Center`;var V=`/images/pexels-fabriziovelez-14980647.jpg`,H=280,U={height:`100dvh`,width:`100%`,background:`linear-gradient(180deg, #eef3f8 0%, #f7fbff 100%)`,overflow:`hidden`},W={width:`100%`,height:`100%`,objectFit:`cover`,display:`block`},G=`
.login-split-grid {
  container-type: inline-size;
  container-name: login-split;
  min-height: 100dvh;
  width: 100%;
}
.login-split-image {
  width: 100%;
  min-height: 100dvh;
  order: 0;
}
.login-split-form {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100dvh;
  padding: var(--spacing-8);
}
.login-split-panel {
  width: min(560px, 100%);
  padding: var(--spacing-10);
  border-radius: 28px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 24px 80px rgba(15, 23, 42, 0.16);
}
.login-split-logo {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  margin-bottom: var(--spacing-6);
}
.login-split-logo img {
  height: 40px;
  width: auto;
}
.login-split-title {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--spacing-2);
  margin-bottom: var(--spacing-5);
}
@container login-split (max-width: 899px) {
  .login-split-form {
    padding: var(--spacing-6);
  }
  .login-split-panel {
    width: 100%;
    border-radius: 20px;
    box-shadow: none;
    background: rgba(255, 255, 255, 0.98);
  }
}
@container login-split (max-width: 679px) {
  .login-split-image {
    order: -1;
    min-height: 280px;
  }
  .login-split-grid {
    padding: 0;
  }
}
`;function K(){let e=i(),t=r({from:`/login`}),n=v(),a=m(e=>e.setUser),[o,c]=(0,x.useState)(`admin`),[d,S]=(0,x.useState)(`123456`),[w,D]=(0,x.useState)(!1),[O,k]=(0,x.useState)(!1),[A,j]=(0,x.useState)(!1);return(0,E.jsxs)(`div`,{style:U,children:[(0,E.jsx)(`style`,{children:G}),(0,E.jsxs)(L,{columns:{minWidth:H,repeat:`fit`},gap:0,align:`stretch`,style:{height:`100%`,width:`100%`},className:`login-split-grid`,children:[(0,E.jsx)(`div`,{className:`login-split-image`,children:(0,E.jsx)(g,{variant:`transparent`,padding:0,width:`100%`,height:`100%`,children:(0,E.jsx)(`img`,{style:W,src:V,alt:`高科技城市 AI 主题封面`})})}),(0,E.jsx)(`div`,{className:`login-split-form`,children:(0,E.jsxs)(`div`,{className:`login-split-panel`,children:[(0,E.jsx)(`div`,{className:`login-split-logo`,children:(0,E.jsx)(`img`,{src:`/frameflow_logo_full.png`,alt:`FrameFlow 标志`})}),(0,E.jsxs)(h,{gap:4,style:{height:`100%`},children:[(0,E.jsxs)(`div`,{className:`login-split-title`,children:[(0,E.jsx)(l,{type:`display-1`,as:`h2`,children:`欢迎来到 FrameFlow`}),(0,E.jsx)(l,{type:`body`,color:`secondary`,size:`sm`,children:`登录你的 FrameFlow 帐户，开启智能巡帧体验`})]}),(0,E.jsx)(T,{size:`fill`,children:(0,E.jsx)(B,{axis:`vertical`,height:`100%`,children:A?(0,E.jsx)(b,{title:`登录成功`,description:`正在跳转到你的工作区...`,icon:(0,E.jsx)(s,{icon:C,size:`lg`})}):(0,E.jsxs)(h,{gap:4,hAlign:`stretch`,width:`100%`,children:[(0,E.jsx)(_,{label:`用户名`,isLabelHidden:!0,type:`email`,placeholder:`请输入邮箱地址`,value:o,onChange:c,size:`lg`}),(0,E.jsx)(h,{gap:1,children:(0,E.jsx)(_,{label:`密码`,isLabelHidden:!0,placeholder:`请输入密码`,type:`password`,value:d,onChange:e=>{S(e),D(!1)},size:`lg`,status:w?{type:`error`,message:`用户名或密码错误，请重试。`}:void 0})}),(0,E.jsx)(u,{label:`登录`,variant:`primary`,size:`lg`,isLoading:O,onClick:async()=>{if(!o.trim()||!d){n({body:`请输入用户名和密码`,type:`error`}),D(!0);return}D(!1),k(!0);try{let r=await y(o,d);a(r.user),n({body:`登录成功`,type:`info`}),j(!0);let i=t.redirect||`/devices`;await e({to:i})}catch(e){n({body:f(e,`登录失败`),type:`error`})}finally{k(!1)}}})]})})}),!A&&(0,E.jsxs)(l,{type:`supporting`,color:`secondary`,children:[`如果没有账号，`,` `,(0,E.jsx)(p,{href:`#`,type:`supporting`,children:`立即注册`}),`。`]})]})]})})]}),(0,E.jsx)(h,{hAlign:`center`,style:{padding:`0 var(--spacing-8)`},children:(0,E.jsxs)(l,{type:`supporting`,color:`secondary`,children:[`点击继续即表示您同意我们的`,` `,(0,E.jsx)(p,{href:`#`,type:`supporting`,children:`服务条款`}),` `,`和`,` `,(0,E.jsx)(p,{href:`#`,type:`supporting`,children:`隐私政策`}),`。`]})})]})}var q=K;export{q as component};