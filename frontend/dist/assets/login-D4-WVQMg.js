import{a as e,n as t,t as n}from"./jsx-runtime-D3jfb0Ew.js";import{t as r}from"./useSearch-DK_lvnWB.js";import{t as i}from"./useNavigate-CaYMtyO9.js";import{C as a,f as o,l as s,p as c,q as l,u}from"./padding.stylex-C9XStNzP.js";import{g as d,t as f}from"./http-CJFmveb_.js";import{t as p}from"./Link-knlNFyb4.js";import{K as m,R as h,l as g,r as _,x as v}from"./index-4JUftChx.js";import{n as y,t as b}from"./TextInput-BEXZ7GMf.js";import{t as x}from"./Card-DR91JbB9.js";import{t as S}from"./useToast-fX7GE_-3.js";import{t as C}from"./auth-api-DzJVG4Qj.js";import{t as w}from"./EmptyState-OrfHKQhj.js";var T=e(t());function E({title:e,titleId:t,...n},r){return T.createElement(`svg`,Object.assign({xmlns:`http://www.w3.org/2000/svg`,fill:`none`,viewBox:`0 0 24 24`,strokeWidth:1.5,stroke:`currentColor`,"aria-hidden":`true`,"data-slot":`icon`,ref:r,"aria-labelledby":t},n),e?T.createElement(`title`,{id:t},e):null,T.createElement(`path`,{strokeLinecap:`round`,strokeLinejoin:`round`,d:`M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z`}))}var D=T.forwardRef(E),O={scrollable:{kVQacm:`xysyzu8`,$$css:!0}};function k({crossAlignSelf:e,size:t,isScrollable:n,as:r=`div`,xstyle:i,className:o,style:s,children:c,ref:u,...f}){let p=l(...d({crossAlignSelf:e,size:t}),n&&O.scrollable,i);return(0,T.createElement)(r,{ref:u,...m(a(`stack-item`,{size:t}),p,o,s),...f},c)}k.displayName=`StackItem`;var A=n(),j={grid:{k1xSpc:`xrvj5dj`,$$css:!0}},M={kJBjZk:`xhij9v2`,$$css:!0},N={templateColumns:e=>[{kumcoG:e==null?e:`xqketvx`,$$css:!0},{"--x-gridTemplateColumns":e??void 0}],autoRows:e=>[M,{"--x-gridAutoRows":`${e}px`==null?void 0:`${e}px`}]},P={start:{kGNEyG:`x7a106z`,$$css:!0},center:{kGNEyG:`x6s0dn4`,$$css:!0},end:{kGNEyG:`xpqajaz`,$$css:!0},stretch:{kGNEyG:`x1qjc9v5`,$$css:!0}},F={start:{kAPf3g:`x619ttb`,$$css:!0},center:{kAPf3g:`x1o2pa38`,$$css:!0},end:{kAPf3g:`x4xo5sw`,$$css:!0},stretch:{kAPf3g:`xl4xnwh`,$$css:!0}},I={0:{kOIVth:`xsn7fz1`,$$css:!0},1:{kOIVth:`xzye2dw`,$$css:!0},2:{kOIVth:`x1txdalj`,$$css:!0},3:{kOIVth:`xjcht0a`,$$css:!0},4:{kOIVth:`x18g69wz`,$$css:!0},5:{kOIVth:`x9mgr7n`,$$css:!0},6:{kOIVth:`x1qh66ti`,$$css:!0},8:{kOIVth:`x4t41sb`,$$css:!0},10:{kOIVth:`x3hoi3v`,$$css:!0},"0.5":{kOIVth:`x1lsbc85`,$$css:!0},"1.5":{kOIVth:`x1s4dlld`,$$css:!0}},L={0:{khm7nJ:`x6yxi7o`,$$css:!0},1:{khm7nJ:`x1ngg2t4`,$$css:!0},2:{khm7nJ:`x1x7z4sm`,$$css:!0},3:{khm7nJ:`x4olc9o`,$$css:!0},4:{khm7nJ:`xtx9w7w`,$$css:!0},5:{khm7nJ:`x1iu6piu`,$$css:!0},6:{khm7nJ:`xczp1bk`,$$css:!0},8:{khm7nJ:`xgx0vcf`,$$css:!0},10:{khm7nJ:`x1xpicb7`,$$css:!0},"0.5":{khm7nJ:`x1tw44j4`,$$css:!0},"1.5":{khm7nJ:`xhq53yo`,$$css:!0}},R={0:{k1C7PZ:`x1o57wo1`,$$css:!0},1:{k1C7PZ:`x1lfs0n9`,$$css:!0},2:{k1C7PZ:`xak3so`,$$css:!0},3:{k1C7PZ:`xewh9hi`,$$css:!0},4:{k1C7PZ:`xty4p9g`,$$css:!0},5:{k1C7PZ:`x1eqhezk`,$$css:!0},6:{k1C7PZ:`x3qlgwd`,$$css:!0},8:{k1C7PZ:`xicv188`,$$css:!0},10:{k1C7PZ:`x1p37tyl`,$$css:!0},"0.5":{k1C7PZ:`x1kihgfc`,$$css:!0},"1.5":{k1C7PZ:`x1thn6ci`,$$css:!0}},z={0:`--spacing-0`,.5:`--spacing-0-5`,1:`--spacing-1`,1.5:`--spacing-1-5`,2:`--spacing-2`,3:`--spacing-3`,4:`--spacing-4`,5:`--spacing-5`,6:`--spacing-6`,8:`--spacing-8`,10:`--spacing-10`};function B(e,t,n,r,i){let a=i==null?r==null?null:z[r]:z[i];return`repeat(${n}, minmax(${`min(100%, max(${e}px, ${a?`calc((100% - ${t-1} * var(${a})) / ${t})`:`calc(100% / ${t})`}))`}, 1fr))`}function V({columns:e,rowHeight:t,width:n,height:r,maxWidth:i,minHeight:o,gap:s,rowGap:c,columnGap:u,align:d,justify:f,xstyle:p,className:h,style:g,children:_,ref:v,...y}){let b;if(typeof e==`object`&&e){let t=e.repeat===`fit`?`auto-fit`:`auto-fill`;b=e.max!=null&&e.max>0?B(e.minWidth,e.max,t,s,u):`repeat(${t}, minmax(${e.minWidth}px, 1fr))`}else b=typeof e==`number`&&e>0?`repeat(${e}, 1fr)`:`1fr`;let x={...n!=null&&{width:typeof n==`number`?`${n}px`:n},...r!=null&&{height:typeof r==`number`?`${r}px`:r},...i!=null&&{maxWidth:typeof i==`number`?`${i}px`:i},...o!=null&&{minHeight:typeof o==`number`?`${o}px`:o}};return(0,A.jsx)(`div`,{ref:v,...m(a(`grid`,{columns:typeof e==`number`?e:void 0,gap:s,align:d,justify:f}),l(j.grid,N.templateColumns(b),t!=null&&N.autoRows(t),s!=null&&I[s],c!=null&&L[c],u!=null&&R[u],d!=null&&P[d],f!=null&&F[f],p),h,{...g,...x}),...y,children:_})}V.displayName=`Grid`;var H={base:{k1xSpc:`x78zum5`,$$css:!0},inline:{k1xSpc:`x3nfvp2`,$$css:!0},alignItemsCenter:{kGNEyG:`x6s0dn4`,$$css:!0},justifyContentCenter:{kjj79g:`xl56j7k`,$$css:!0}},U={sizing:(e,t,n,r)=>[{kzqmXN:e==null?e:`x5lhr3w`,kZKoxP:t==null?t:`x16ye13r`,ks0D6T:n==null?n:`xf68679`,kAzted:r==null?r:`x82snj4`,$$css:!0},{"--x-width":(e=>typeof e==`number`?e+`px`:e??void 0)(e),"--x-height":(e=>typeof e==`number`?e+`px`:e??void 0)(t),"--x-maxWidth":(e=>typeof e==`number`?e+`px`:e??void 0)(n),"--x-minHeight":(e=>typeof e==`number`?e+`px`:e??void 0)(r)}]};function W({axis:e=`both`,width:t,height:n,maxWidth:r,minHeight:i,padding:d,paddingInline:f,paddingInlineStart:p,paddingInlineEnd:h,paddingBlock:g,paddingBlockStart:_,paddingBlockEnd:v,isInline:y=!1,children:b,xstyle:x,className:S,style:C,ref:w,...T}){let E=p??f??d,D=h??f??d,O=_??g??d,k=v??g??d,j=m(a(`center`,{axis:e}),l(y?H.inline:H.base,(e===`both`||e===`vertical`)&&H.alignItemsCenter,(e===`both`||e===`horizontal`)&&H.justifyContentCenter,U.sizing(t??null,n??null,r??null,i??null),E!=null&&c[E],D!=null&&o[D],O!=null&&u[O],k!=null&&s[k],x),S,C);return(0,A.jsx)(`div`,{ref:w,...T,...j,children:b})}W.displayName=`Center`;var G=`/images/pexels-fabriziovelez-14980647.jpg`,K=280,q={height:`100dvh`,width:`100%`,background:`linear-gradient(180deg, #eef3f8 0%, #f7fbff 100%)`,overflow:`hidden`},J={width:`100%`,height:`100%`,objectFit:`cover`,display:`block`},Y=`
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
`;function X(){let e=i(),t=r({from:`/login`}),n=S(),a=_(e=>e.setUser),[o,s]=(0,T.useState)(`admin`),[c,l]=(0,T.useState)(`123456`),[u,d]=(0,T.useState)(!1),[m,E]=(0,T.useState)(!1),[O,j]=(0,T.useState)(!1);return(0,A.jsxs)(`div`,{style:q,children:[(0,A.jsx)(`style`,{children:Y}),(0,A.jsxs)(V,{columns:{minWidth:K,repeat:`fit`},gap:0,align:`stretch`,style:{height:`100%`,width:`100%`},className:`login-split-grid`,children:[(0,A.jsx)(`div`,{className:`login-split-image`,children:(0,A.jsx)(x,{variant:`transparent`,padding:0,width:`100%`,height:`100%`,children:(0,A.jsx)(`img`,{style:J,src:G,alt:`高科技城市 AI 主题封面`})})}),(0,A.jsx)(`div`,{className:`login-split-form`,children:(0,A.jsxs)(`div`,{className:`login-split-panel`,children:[(0,A.jsx)(`div`,{className:`login-split-logo`,children:(0,A.jsx)(`img`,{src:`/frameflow_logo_full.png`,alt:`FrameFlow 标志`})}),(0,A.jsxs)(y,{gap:4,style:{height:`100%`},children:[(0,A.jsxs)(`div`,{className:`login-split-title`,children:[(0,A.jsx)(v,{type:`display-1`,as:`h2`,children:`欢迎来到 FrameFlow`}),(0,A.jsx)(v,{type:`body`,color:`secondary`,size:`sm`,children:`登录你的 FrameFlow 帐户，开启智能巡帧体验`})]}),(0,A.jsx)(k,{size:`fill`,children:(0,A.jsx)(W,{axis:`vertical`,height:`100%`,children:O?(0,A.jsx)(w,{title:`登录成功`,description:`正在跳转到你的工作区...`,icon:(0,A.jsx)(h,{icon:D,size:`lg`})}):(0,A.jsxs)(y,{gap:4,hAlign:`stretch`,width:`100%`,children:[(0,A.jsx)(b,{label:`用户名`,isLabelHidden:!0,type:`email`,placeholder:`请输入邮箱地址`,value:o,onChange:s,size:`lg`}),(0,A.jsx)(y,{gap:1,children:(0,A.jsx)(b,{label:`密码`,isLabelHidden:!0,placeholder:`请输入密码`,type:`password`,value:c,onChange:e=>{l(e),d(!1)},size:`lg`,status:u?{type:`error`,message:`用户名或密码错误，请重试。`}:void 0})}),(0,A.jsx)(g,{label:`登录`,variant:`primary`,size:`lg`,isLoading:m,onClick:async()=>{if(!o.trim()||!c){n({body:`请输入用户名和密码`,type:`error`}),d(!0);return}d(!1),E(!0);try{let r=await C(o,c);a(r.user),n({body:`登录成功`,type:`info`}),j(!0);let i=t.redirect||`/devices`;await e({to:i})}catch(e){n({body:f(e,`登录失败`),type:`error`})}finally{E(!1)}}})]})})}),!O&&(0,A.jsxs)(v,{type:`supporting`,color:`secondary`,children:[`如果没有账号，`,` `,(0,A.jsx)(p,{href:`#`,type:`supporting`,children:`立即注册`}),`。`]})]})]})})]}),(0,A.jsx)(y,{hAlign:`center`,style:{padding:`0 var(--spacing-8)`},children:(0,A.jsxs)(v,{type:`supporting`,color:`secondary`,children:[`点击继续即表示您同意我们的`,` `,(0,A.jsx)(p,{href:`#`,type:`supporting`,children:`服务条款`}),` `,`和`,` `,(0,A.jsx)(p,{href:`#`,type:`supporting`,children:`隐私政策`}),`。`]})})]})}var Z=X;export{Z as component};